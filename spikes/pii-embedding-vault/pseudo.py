"""Pseudonymize text against the vault, in three representations, and rebuild it exactly.

  token_whole  every value is one opaque reference, [[PII_<KIND>_<32 hex>]] -- the spec's form
  token_parts  a person is its given name's reference and its surname's; a company keeps its
               sector words and references its proper words -- "la Ferraris" then matches
  surrogate    the same parts, but each one is a consistent invented value of the same kind

The vault decides every replacement: the same value, in any case, gets the same entry for the
identity. A reference carries the case it replaced (_U upper, _L lower, _M<mask> mixed), so the
rebuild is byte-exact. A surrogate is for the embedding exit only, which never comes back.
"""
import hashlib, hmac, re, unicodedata

SECTOR_WORDS = {"ferramenta", "studio", "legale", "officina", "immobiliare", "panificio", "elettrodomestici",
                "arredamenti", "banca", "di", "credito", "&", "associati", "s.r.l.", "s.p.a.", "s.n.c.", "piemonte"}
SINGLE_WORD_KINDS = {"GIVEN", "SURNAME", "ORGWORD"}
STRUCTURED = {"EMAIL", "PHONE", "STREET", "CF", "IBAN", "PIVA"}

FAKE = {
    "GIVEN_F": ["Ludovica", "Tiziana", "Ginevra", "Ilaria", "Noemi", "Beatrice", "Camilla", "Adelina", "Ornella",
                "Rebecca", "Letizia", "Virginia", "Melania", "Fiorella", "Annalisa"],
    "GIVEN_M": ["Ottavio", "Ermanno", "Benedetto", "Corrado", "Vittorio", "Raffaele", "Ettore", "Gualtiero",
                "Severino", "Fulvio", "Ambrogio", "Dario", "Amedeo", "Oreste", "Cesare"],
    "SURNAME": ["Bertolaso", "Zanetti", "Calligaris", "Pellizzari", "Trevisan", "Marchesini", "Bonaventura",
                "Castellani", "Guidetti", "Sartorello", "Fumagalli", "Righetti", "Vianello", "Cappelletti", "Mezzadri",
                "Paolucci", "Rinaldini", "Tagliaferri", "Zuccarello", "Benedettini", "Capuzzo", "Dalmasso", "Garrone",
                "Lusardi", "Mombello", "Novelli", "Orsenigo", "Pastorino", "Quilici", "Scarpellini"],
    "ORGWORD": ["Novatek", "Brentana", "Selvaggi", "Orobica", "Lariana", "Ferronia", "Cimbra", "Doralis", "Valmora",
                "Sabelli", "Arvenis", "Telvana"],
    "STREET": ["Via dei Tigli", "Via Pellico", "Corso Belgio", "Via Bardonecchia", "Via Asti", "Via Bligny",
               "Via Lessona", "Viale Monza", "Via Ormea", "Corso Tassoni", "Via Cibrario", "Via Frejus", "Via Susa",
               "Via Peyron", "Via Nota", "Corso Racconigi", "Via Pinelli", "Via Fabro"],
}
ODD = dict(zip("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ",
               [1, 0, 5, 7, 9, 13, 15, 17, 19, 21, 1, 0, 5, 7, 9, 13, 15, 17, 19, 21, 2, 4, 18, 20,
                11, 3, 6, 8, 12, 14, 16, 10, 22, 25, 24, 23]))
EVEN = {**{str(d): d for d in range(10)}, **{chr(65 + i): i for i in range(26)}}
# OpenRouter's Sensitive Info redaction, with perfect detection: its placeholders name the type only
# (docs guides/features/guardrails/sensitive-info, read 2026-10-20). No preset covers companies, and
# codice fiscale, IBAN and partita IVA need a custom pattern, whose placeholder is [REDACTED].
OPENROUTER_LABEL = {"PERSON": "[PERSON_NAME]", "GIVEN": "[PERSON_NAME]", "SURNAME": "[PERSON_NAME]",
                    "STREET": "[ADDRESS]", "EMAIL": "[EMAIL]", "PHONE": "[PHONE]",
                    "CF": "[REDACTED]", "IBAN": "[REDACTED]", "PIVA": "[REDACTED]"}
TOKEN_RE = re.compile(r"\[\[(PII_[A-Z]+_[0-9a-f]{32})(?:_(U|L|M[0-9a-f]+))?\]\]")


BELFIORE = ["L219", "F839", "D969", "F205", "D205", "H727", "A124", "H501", "B157", "G273", "C351", "E506"]


def _letters(s):
    s = "".join(c for c in unicodedata.normalize("NFKD", s.upper()) if "A" <= c <= "Z")
    return [c for c in s if c not in "AEIOU"], [c for c in s if c in "AEIOU"]


def cf_name_code(given, surname):
    """The six letters a codice fiscale takes from the surname and the given name."""
    cons, vows = _letters(surname)
    sc = (cons + vows + ["X"] * 3)[:3]
    cons, vows = _letters(given)
    gc = [cons[0], cons[2], cons[3]] if len(cons) >= 4 else (cons + vows + ["X"] * 3)[:3]
    return "".join(sc + gc)


def slug(s):
    s = unicodedata.normalize("NFKD", s)
    return "".join(c for c in s.lower() if c.isalnum() and ord(c) < 128)


def case_flag(surface, canonical):
    if surface == canonical:
        return ""
    if surface == canonical.upper():
        return "U"
    if surface == canonical.lower():
        return "L"
    mask = sum(1 << i for i, c in enumerate(surface) if c.isupper())
    return f"M{mask:x}"


def apply_case(text, flag):
    if flag == "":
        return text
    if flag == "U":
        return text.upper()
    if flag == "L":
        return text.lower()
    mask = int(flag[1:], 16)
    return "".join(c.upper() if mask >> i & 1 else c.lower() for i, c in enumerate(text))


class Pseudonymizer:
    def __init__(self, vault, entities):
        self.v = vault
        self.kind_of = {}     # casefold value -> (kind, canonical value)
        self.parts = {}       # PERSON canonical -> (given, surname)
        self.female = {e["value"]: e.get("female", False) for e in entities if e["kind"] == "GIVEN"}
        for e in entities:
            self.kind_of[e["value"].casefold()] = (e["kind"], e["value"])
        persons = [e["value"] for e in entities if e["kind"] == "PERSON"]
        givens = [e["value"] for e in entities if e["kind"] == "GIVEN"]
        for full in persons:  # the given name is the longest known given name the full name starts with
            g = max((g for g in givens if full.startswith(g + " ")), key=len)
            self.parts[full] = (g, full[len(g) + 1:])
        for e in entities:  # a company's proper words are entities of their own
            if e["kind"] == "ORG":
                for w in e["value"].split():
                    if w.casefold() not in SECTOR_WORDS and w.casefold() not in self.kind_of:
                        self.kind_of[w.casefold()] = ("ORGWORD", w)
        self.real = {v for v in self.kind_of}
        values = sorted(self.kind_of, key=len, reverse=True)
        self.find = re.compile(r"(?<!\w)(" + "|".join(re.escape(v) for v in values) + r")(?!\w)", re.IGNORECASE)
        for cf in values:  # every entry is stored before any text that uses it leaves
            self.entry(*self.kind_of[cf])

    # -- surrogates ---------------------------------------------------------------------------------
    def _pick(self, kind, value, attempt, pool):
        h = int(self.v.mac("pii-pick", f"{kind}\x00{value.casefold()}\x00{attempt}"), 16)
        return pool[h % len(pool)]

    def _digits(self, kind, value, attempt, n):
        h = self.v.mac("pii-digits", f"{kind}\x00{value.casefold()}\x00{attempt}")
        return "".join(str(int(c, 16) % 10) for c in h)[:n]

    def surrogate_for(self, kind, value, attempt):
        if kind == "GIVEN":
            pool = [x for x in FAKE["GIVEN_F" if self.female.get(value) else "GIVEN_M"] if x.casefold() not in self.real]
            return self._pick(kind, value, attempt, pool)
        if kind in ("SURNAME", "ORGWORD"):
            pool = [x for x in FAKE[kind] if x.casefold() not in self.real]
            return self._pick(kind, value, attempt, pool)
        if kind == "PERSON":
            g, s = self.parts[value]
            return self.entry("GIVEN", g)["sur"] + " " + self.entry("SURNAME", s)["sur"]
        if kind == "ORG":
            return " ".join(self._word_sur(w) for w in value.split())
        if kind == "STREET":
            return self._pick(kind, value, attempt, FAKE["STREET"]) + " " + str(1 + int(self._digits(kind, value, attempt, 3)) % 199)
        if kind == "PHONE":
            d = self._digits(kind, value, attempt, 9)
            return f"+39 3{d[:2]} {d[2:5]} {d[5:9]}"
        if kind == "EMAIL":
            return self._email_sur(value, attempt)
        if kind == "IBAN":
            d = self._digits(kind, value, attempt, 22)
            bban = chr(65 + int(d[:2]) % 26) + d[2:22]
            num = "".join(str(int(c, 36)) for c in bban + "IT00")
            return f"IT{98 - int(num) % 97:02d}{bban}"
        if kind == "PIVA":
            d = self._digits(kind, value, attempt, 10)
            s = sum((int(c) * 2 - 9 if int(c) * 2 > 9 else int(c) * 2) if i % 2 else int(c) for i, c in enumerate(d))
            return d + str((10 - s % 10) % 10)
        if kind == "CF":
            # A real codice fiscale spells its owner's name; so does the surrogate, with the surrogate name.
            h = self.v.mac("pii-cf", f"{value.casefold()}\x00{attempt}")
            letters, female = "".join(chr(65 + int(h[i:i + 2], 16) % 26) for i in range(0, 12, 2)), False
            for g, s in self.parts.values():
                if cf_name_code(g, s) == value[:6].upper():
                    letters = cf_name_code(self.entry("GIVEN", g)["sur"], self.entry("SURNAME", s)["sur"])
                    female = self.female.get(g, False)
                    break
            dd = self._digits(kind, value, attempt, 8)
            body = letters + dd[:2] + "ABCDEHLMPRST"[int(dd[2:4]) % 12] + \
                f"{1 + int(dd[4:6]) % 28 + (40 if female else 0):02d}" + BELFIORE[int(dd[6:8]) % len(BELFIORE)]
            total = sum(ODD[c] if i % 2 == 0 else EVEN[c] for i, c in enumerate(body))
            return body + chr(65 + total % 26)
        raise ValueError(kind)

    def _word_sur(self, w):
        hit = self.kind_of.get(w.casefold())
        if hit and hit[0] in ("SURNAME", "ORGWORD"):
            return apply_case(self.entry(*hit)["sur"], case_flag(w, hit[1]))
        return w

    def _email_sur(self, value, attempt):
        local, domain = value.split("@", 1)
        names = sorted(((slug(c[1]), c) for c in self.kind_of.values() if c[0] in SINGLE_WORD_KINDS and len(slug(c[1])) >= 4),
                       key=lambda x: -len(x[0]))
        def swap(part):
            for s, (k, canon) in names:
                if s in part:
                    part = part.replace(s, slug(self.entry(k, canon)["sur"]))
            return part
        new = swap(local) + "@" + swap(domain)
        return new if new != value else f"utente{self._digits('EMAIL', value, attempt, 5)}@" + domain

    def entry(self, kind, value):
        return self.v.entry(kind, value, self.surrogate_for)

    # -- text ---------------------------------------------------------------------------------------
    def _ref(self, kind, canon, surface):
        return f"[[{self.entry(kind, canon)['token']}{'_' if case_flag(surface, canon) else ''}{case_flag(surface, canon)}]]"

    def _render(self, kind, canon, surface, rep):
        if rep == "openrouter_redact":
            return OPENROUTER_LABEL.get(kind, surface)
        if rep == "token_whole" or kind in STRUCTURED:
            if rep == "surrogate":
                return self.entry(kind, canon)["sur"]
            return self._ref(kind, canon, surface)
        flag = case_flag(surface, canon)
        if kind == "PERSON":
            g, s = self.parts[canon]
            gs, ss = surface[:len(g)], surface[len(g) + 1:]
            if rep == "surrogate":
                return apply_case(self.entry("GIVEN", g)["sur"], case_flag(gs, g) if flag[:1] != "M" else "") + " " + \
                    apply_case(self.entry("SURNAME", s)["sur"], case_flag(ss, s) if flag[:1] != "M" else "")
            return self._ref("GIVEN", g, gs) + " " + self._ref("SURNAME", s, ss)
        if kind == "ORG":
            words, out = surface.split(" "), []
            for w, cw in zip(words, canon.split(" ")):
                hit = self.kind_of.get(cw.casefold())
                if hit and hit[0] in ("SURNAME", "ORGWORD"):
                    out.append(apply_case(self.entry(*hit)["sur"], case_flag(w, cw)) if rep == "surrogate"
                               else self._ref(hit[0], hit[1], w))
                else:
                    out.append(w)
            return " ".join(out)
        if rep == "surrogate":
            return apply_case(self.entry(kind, canon)["sur"], flag if flag[:1] != "M" else "")
        return self._ref(kind, canon, surface)

    def pseudonymize(self, text, rep):
        if rep == "clear":
            return text, 0
        count = 0

        def sub(m):
            nonlocal count
            surface = m.group(0)
            kind, canon = self.kind_of[surface.casefold()]
            if kind in SINGLE_WORD_KINDS and not surface[:1].isupper():
                return surface  # a lower-case single word is an ordinary word, not a name
            count += 1
            return self._render(kind, canon, surface, rep)

        return self.find.sub(sub, text), count

    def residue(self, text):
        """Known personal values still readable in text: the leak check."""
        out = []
        for m in self.find.finditer(text):
            kind, _ = self.kind_of[m.group(0).casefold()]
            if kind in SINGLE_WORD_KINDS and not m.group(0)[:1].isupper():
                continue
            out.append(m.group(0))
        return out

    def rebuild_tokens(self, text):
        def sub(m):
            e = self.v.get("token", m.group(1))
            if e is None:
                raise KeyError(f"unknown reference {m.group(1)}")
            return apply_case(e["value"], m.group(2) or "")
        return TOKEN_RE.sub(sub, text)

    def rebuild_surrogates(self, text):
        surs = {}
        for e in self.v.by_digest.values():
            surs[e["sur"].casefold()] = e
        rx = re.compile(r"(?<!\w)(" + "|".join(re.escape(s) for s in sorted(surs, key=len, reverse=True)) + r")(?!\w)", re.IGNORECASE)

        def sub(m):
            e = self.v.get("sur_digest", self.v.sur_digest(surs[m.group(0).casefold()]["kind"], m.group(0)))
            return apply_case(e["value"], case_flag(m.group(0), e["sur"]))
        return rx.sub(sub, text)
