"""A synthetic Italian corpus with personal data, and questions whose answer depends on a name.

Every person, company, address, email, phone, codice fiscale, IBAN and partita IVA here is
invented; checksums are valid so a pattern detector would accept them. Several documents share a
template and differ only by who they are about (four complaints, four invoices, three leases, three
hirings, three refunds), so a question naming the person can only be answered by the name.

Writes corpus.json: docs, entities (the ground truth of what is personal) and questions.
"""
import json, os

HERE = os.path.dirname(os.path.abspath(__file__))

ODD = dict(zip("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ",
               [1, 0, 5, 7, 9, 13, 15, 17, 19, 21, 1, 0, 5, 7, 9, 13, 15, 17, 19, 21, 2, 4, 18, 20,
                11, 3, 6, 8, 12, 14, 16, 10, 22, 25, 24, 23]))
EVEN = {**{str(d): d for d in range(10)}, **{chr(65 + i): i for i in range(26)}}
MONTHS = "ABCDEHLMPRST"
BELFIORE = {"Torino": "L219", "Napoli": "F839", "Genova": "D969", "Milano": "F205", "Cuneo": "D205",
            "Saluzzo": "H727", "Alba": "A124", "Roma": "H501"}


def _letters(s):
    s = "".join(c for c in s.upper().replace("Ò", "O").replace("À", "A") if c.isalpha())
    cons = [c for c in s if c not in "AEIOU"]
    vows = [c for c in s if c in "AEIOU"]
    return cons, vows


def codice_fiscale(given, surname, year, month, day, female, city):
    cons, vows = _letters(surname)
    sc = (cons + vows + ["X"] * 3)[:3]
    cons, vows = _letters(given)
    gc = [cons[0], cons[2], cons[3]] if len(cons) >= 4 else (cons + vows + ["X"] * 3)[:3]
    body = "".join(sc + gc) + f"{year % 100:02d}" + MONTHS[month - 1] + f"{day + (40 if female else 0):02d}" + BELFIORE[city]
    total = sum(ODD[c] if i % 2 == 0 else EVEN[c] for i, c in enumerate(body))
    return body + chr(65 + total % 26)


def iban(cin, abi, cab, account):
    bban = f"{cin}{abi}{cab}{account}"
    digits = "".join(str(int(c, 36)) for c in bban + "IT00")
    return f"IT{98 - int(digits) % 97:02d}{bban}"


def partita_iva(seven, office):
    d = f"{seven}{office}"
    s = 0
    for i, c in enumerate(d):
        n = int(c)
        if i % 2 == 1:
            n *= 2
            n = n - 9 if n > 9 else n
        s += n
    return d + str((10 - s % 10) % 10)


P = {}  # key -> person


def person(key, given, surname, female, email, phone, street, cap_city, city, birth, abi):
    y, m, d = birth
    P[key] = dict(given=given, surname=surname, full=f"{given} {surname}", female=female, email=email, phone=phone,
                  street=street, cap_city=cap_city,
                  cf=codice_fiscale(given, surname, y, m, d, female, city),
                  iban=iban("X", abi, "01000", f"{(y * 7919 + m * 104729 + d * 1299709) % 10**12:012d}"))


person("ferraris", "Giulia", "Ferraris", True, "giulia.ferraris@gmail.com", "+39 347 218 5590", "Via Nizza 48", "10126 Torino", "Torino", (1988, 4, 17), "03069")
person("bellini", "Marco", "Bellini", False, "marco.bellini@libero.it", "+39 335 611 2087", "Corso Francia 112", "10143 Torino", "Torino", (1975, 11, 2), "02008")
person("esposito", "Roberto", "Esposito", False, "roberto.esposito@outlook.it", "+39 328 904 1176", "Via Toledo 256", "80134 Napoli", "Napoli", (1969, 7, 23), "01030")
person("dangelo", "Chiara", "D'Angelo", True, "chiara.dangelo@gmail.com", "+39 340 777 3412", "Via dei Mille 9", "16147 Genova", "Genova", (1992, 2, 9), "05034")
person("deluca", "Nicolò", "De Luca", False, "nicolo.deluca@yahoo.it", "+39 339 152 8803", "Piazza Vittorio Veneto 3", "10124 Torino", "Torino", (1985, 9, 30), "03069")
person("colombo", "Maria Grazia", "Colombo", True, "mariagrazia.colombo@gmail.com", "+39 348 330 9921", "Via Manzoni 21", "20121 Milano", "Milano", (1961, 5, 14), "03440")
person("mrossi", "Mario", "Rossi", False, "mario.rossi@tiscali.it", "+39 333 480 1265", "Via Po 17", "10123 Torino", "Torino", (1979, 1, 8), "02008")
person("martarossi", "Marta", "Rossi", True, "marta.rossi@gmail.com", "+39 346 905 7731", "Via Garibaldi 64", "12100 Cuneo", "Cuneo", (1994, 12, 3), "08450")
person("gallo", "Francesca", "Gallo", True, "francesca.gallo@pec.it", "+39 320 446 0198", "Via XX Settembre 5", "16121 Genova", "Genova", (1990, 6, 21), "05034")
person("fontana", "Luca", "Fontana", False, "luca.fontana@gmail.com", "+39 331 207 6654", "Via Roma 131", "12037 Saluzzo", "Saluzzo", (1997, 3, 11), "08450")
person("moretti", "Alessandro", "Moretti", False, "alessandro.moretti@libero.it", "+39 349 818 2203", "Via Cavour 40", "12051 Alba", "Alba", (1972, 10, 27), "06085")
person("barbieri", "Elena", "Barbieri", True, "elena.barbieri@outlook.it", "+39 338 560 4417", "Viale Mazzini 77", "00195 Roma", "Roma", (1983, 8, 5), "01030")
person("lombardi", "Giorgio", "Lombardi", False, "giorgio.lombardi@gmail.com", "+39 347 091 3368", "Via Sacchi 2", "10128 Torino", "Torino", (1966, 2, 19), "03069")
person("ricci", "Sara", "Ricci", True, "sara.ricci@yahoo.it", "+39 345 672 8890", "Via Bologna 220", "10154 Torino", "Torino", (1991, 11, 29), "02008")

O = {
    "ferramenta": dict(name="Ferramenta Bellini S.r.l.", email="info@ferramentabellini.it", piva=partita_iva("0812345", "001"),
                       iban=iban("K", "03069", "10100", "000004412378"), street="Corso Francia 110"),
    "tecno": dict(name="Tecnoservizi Piemonte S.p.A.", email="amministrazione@tecnoservizipiemonte.it", piva=partita_iva("1093847", "001"),
                  iban=iban("M", "02008", "01046", "000101928374"), street="Via Livorno 60"),
    "conti": dict(name="Studio Legale Conti & Associati", email="segreteria@studiolegaleconti.it", piva=partita_iva("0567281", "006"),
                  iban=iban("B", "01030", "01000", "000063527183"), street="Via Pietro Micca 15"),
    "officina": dict(name="Officina Moretti S.n.c.", email="officina@morettiauto.it", piva=partita_iva("0372615", "004"),
                     iban=iban("P", "06085", "22500", "000000776210"), street="Corso Langhe 33"),
    "immobiliare": dict(name="Immobiliare Lombardi S.r.l.", email="info@immobiliarelombardi.it", piva=partita_iva("1182736", "001"),
                        iban=iban("F", "03069", "01000", "100000063871"), street="Via Sacchi 4"),
    "panificio": dict(name="Panificio Colombo", email="panificiocolombo@gmail.com", piva=partita_iva("0928374", "015"),
                      iban=iban("C", "03440", "01600", "000000552781"), street="Via Manzoni 23"),
    "sereni": dict(name="Elettrodomestici Sereni S.r.l.", email="assistenza@elettrodomesticisereni.it", piva=partita_iva("0743829", "001"),
                   iban=iban("D", "02008", "30030", "000040017265"), street="Strada del Drosso 91"),
    "valtellina": dict(name="Arredamenti Valtellina S.p.A.", email="clienti@arredamentivaltellina.it", piva=partita_iva("0618273", "014"),
                       iban=iban("E", "05696", "11000", "000003378122"), street="Via Stelvio 7"),
}
NEW_IBAN_BARBIERI = iban("Q", "05034", "03200", "000000918273")


def p(k, f):
    return P[k][f]


def o(k, f):
    return O[k][f]


DOCS = {
    "D01": f"""Oggetto: lavatrice rumorosa, ordine n. 4471
Gentile assistenza di {o('sereni','name')},
vi scrivo perché la lavatrice consegnata il mese scorso fa un rumore fortissimo durante la centrifuga e si sposta sul pavimento. Ho già controllato i piedini e il carico. Chiedo l'intervento di un tecnico a domicilio, in {p('ferraris','street')}, {p('ferraris','cap_city')}.
Potete contattarmi al {p('ferraris','phone')} o a {p('ferraris','email')}.
Cordiali saluti,
{p('ferraris','full')}""",
    "D02": f"""Oggetto: consegna divano in ritardo
Spett.le {o('valtellina','name')},
il divano angolare ordinato a luglio è stato consegnato con tre settimane di ritardo rispetto alla data confermata, e ho dovuto prendere due giorni di ferie per attendere il corriere. Chiedo uno sconto sul prezzo pagato come compensazione del disagio.
Distinti saluti,
{p('mrossi','full')} – {p('mrossi','street')}, {p('mrossi','cap_city')} – tel. {p('mrossi','phone')}""",
    "D03": f"""Oggetto: fattura addebitata due volte
Buongiorno {o('tecno','name')},
la fattura n. 2026/0937 per il controllo dell'impianto di condizionamento mi è stata addebitata due volte sulla carta. Vi chiedo il rimborso dell'importo duplicato di 145,00 euro sul mio conto IBAN {p('gallo','iban')}.
Grazie, {p('gallo','full')} ({p('gallo','email')})""",
    "D04": f"""Oggetto: caldaia ancora guasta dopo l'intervento
Gentile {o('tecno','name')},
il vostro tecnico è passato martedì per la caldaia che va in blocco, ma il problema si è ripresentato la sera stessa e la casa è senza acqua calda. Chiedo un nuovo intervento urgente senza costi aggiuntivi.
{p('ricci','full')}, {p('ricci','street')}, {p('ricci','cap_city')}, cell. {p('ricci','phone')}""",
    "D05": f"""FATTURA n. 2026/118 del 14/10/2026
Emittente: {o('ferramenta','name')}, {o('ferramenta','street')}, 10143 Torino – P.IVA {o('ferramenta','piva')}
Cliente: {p('fontana','full')}, {p('fontana','street')}, {p('fontana','cap_city')} – C.F. {p('fontana','cf')}
Descrizione: trapano a percussione, set di viti e tasselli, livella laser.
Totale documento: 412,80 euro IVA inclusa. Pagamento con bonifico su IBAN {o('ferramenta','iban')}.""",
    "D06": f"""FATTURA n. 2026/1012 del 02/10/2026
Emittente: {o('tecno','name')}, {o('tecno','street')}, 10144 Torino – P.IVA {o('tecno','piva')}
Cliente: {p('ferraris','full')}, {p('ferraris','street')}, {p('ferraris','cap_city')} – C.F. {p('ferraris','cf')}
Descrizione: manutenzione annuale della caldaia e analisi dei fumi.
Totale: 180,00 euro. Pagamento su IBAN {o('tecno','iban')}.""",
    "D07": f"""FATTURA n. 88 del 30/09/2026
Emittente: {o('officina','name')}, {o('officina','street')}, 12051 Alba – P.IVA {o('officina','piva')}
Cliente: {p('barbieri','full')}, {p('barbieri','street')}, {p('barbieri','cap_city')}
Descrizione: tagliando completo, sostituzione olio e filtri, pastiglie freni anteriori.
Totale: 365,00 euro. Bonifico su IBAN {o('officina','iban')}.""",
    "D08": f"""FATTURA n. 2026/131 del 18/10/2026
Emittente: {o('ferramenta','name')}, {o('ferramenta','street')}, 10143 Torino – P.IVA {o('ferramenta','piva')}
Cliente: {p('dangelo','full')}, {p('dangelo','street')}, {p('dangelo','cap_city')}
Descrizione: vernice murale bianca 10 litri, rulli e nastro di carta.
Totale: 96,50 euro. Pagamento su IBAN {o('ferramenta','iban')}.""",
    "D09": f"""CONTRATTO DI LOCAZIONE AD USO ABITATIVO
Tra {o('immobiliare','name')}, con sede in {o('immobiliare','street')}, Torino, rappresentata da {p('lombardi','full')} (locatore), e {p('martarossi','full')}, C.F. {p('martarossi','cf')} (conduttrice).
Oggetto: appartamento di tre vani in {p('martarossi','street')}, {p('martarossi','cap_city')}.
Canone mensile: 750,00 euro, da versare entro il giorno 5 di ogni mese sull'IBAN {o('immobiliare','iban')}. Deposito cauzionale di tre mensilità. Durata quattro anni rinnovabili.""",
    "D10": f"""SCRITTURA PRIVATA DI LOCAZIONE DI BOX AUTO
Il sig. {p('esposito','full')}, C.F. {p('esposito','cf')}, concede in locazione al sig. {p('deluca','full')} il box auto n. 12 situato in {p('deluca','street')}, Torino.
Canone: 120,00 euro al mese, pagamento anticipato trimestrale sull'IBAN {p('esposito','iban')}. Durata un anno.""",
    "D11": f"""CONTRATTO DI LOCAZIONE COMMERCIALE
La sig.ra {p('colombo','full')}, C.F. {p('colombo','cf')}, titolare del {o('panificio','name')}, concede in locazione al sig. {p('moretti','full')} il locale commerciale di sua proprietà in {o('panificio','street')}, Milano.
Canone mensile: 1.400,00 euro più IVA. Durata sei anni più sei. Destinazione: rivendita di ricambi auto.""",
    "D12": f"""LETTERA DI ASSUNZIONE
{o('conti','name')} assume la sig.ra {p('gallo','full')}, nata il 21/06/1990, C.F. {p('gallo','cf')}, residente in {p('gallo','street')}, {p('gallo','cap_city')}, con la qualifica di impiegata amministrativa, contratto a tempo indeterminato.
Data di inizio: 3 novembre 2026. Retribuzione annua lorda: 31.000 euro in quattordici mensilità.""",
    "D13": f"""COMUNICAZIONE DI ASSUNZIONE
{o('ferramenta','name')} assume il sig. {p('fontana','full')}, C.F. {p('fontana','cf')}, come magazziniere addetto al ricevimento merci, con contratto a tempo determinato di dodici mesi.
Inizio: 10 novembre 2026. Orario: 40 ore settimanali, turno del mattino.""",
    "D14": f"""CESSAZIONE DEL RAPPORTO DI LAVORO
Si prende atto delle dimissioni volontarie della dipendente {p('ricci','full')}, C.F. {p('ricci','cf')}, tecnico di assistenza presso {o('tecno','name')}.
Ultimo giorno di lavoro: 30 novembre 2026, nel rispetto del preavviso contrattuale. Il trattamento di fine rapporto sarà liquidato con la busta paga di dicembre.""",
    "D15": f"""RICHIESTA DI RIMBORSO SPESE DI TRASFERTA
Richiedente: {p('deluca','full')}, C.F. {p('deluca','cf')}
Trasferta a Bologna dal 6 all'8 ottobre 2026 per la fiera dell'edilizia: treno, due notti in albergo e pasti.
Importo richiesto: 238,40 euro, da accreditare sull'IBAN {p('deluca','iban')}.""",
    "D16": f"""RICHIESTA DI RIMBORSO
Richiedente: {p('barbieri','full')} ({p('barbieri','email')})
Motivo: corso di formazione sulla contabilità digitale, tre giornate online, con attestato allegato.
Importo: 450,00 euro. Accredito sull'IBAN {p('barbieri','iban')}.""",
    "D17": f"""NOTA SPESE
Richiedente: {p('lombardi','full')}, {o('immobiliare','name')}
Pranzo di lavoro con due clienti interessati all'acquisto di un attico, ristorante in centro a Torino.
Importo: 96,00 euro. Rimborso sull'IBAN {p('lombardi','iban')}.""",
    "D18": f"""SCHEDA CLIENTE
Nome: {p('ferraris','full')}
Indirizzo: {p('ferraris','street')}, {p('ferraris','cap_city')}
Telefono: {p('ferraris','phone')} – Email: {p('ferraris','email')}
Codice fiscale: {p('ferraris','cf')}
Note: preferisce essere contattata nel primo pomeriggio, dopo le 14. Cliente dal 2021, due interventi di assistenza.""",
    "D19": f"""SCHEDA CLIENTE
Nome: {p('esposito','full')}
Indirizzo: {p('esposito','street')}, {p('esposito','cap_city')}
Cellulare: {p('esposito','phone')} – Email: {p('esposito','email')}
Note: idraulico di professione; ha chiesto un preventivo per il rifacimento completo del bagno.""",
    "D20": f"""SCHEDA CLIENTE
Nome: {p('dangelo','full')}
Indirizzo: {p('dangelo','street')}, {p('dangelo','cap_city')}
Telefono: {p('dangelo','phone')} – Email: {p('dangelo','email')}
Note: interessata ai corsi serali di pittura e decorazione; acquista spesso vernici.""",
    "D21": f"""SCHEDA CLIENTE
Nome: {p('mrossi','full')}
Indirizzo: {p('mrossi','street')}, {p('mrossi','cap_city')}
Telefono: {p('mrossi','phone')} – Email: {p('mrossi','email')}
Note: cliente dal 2019, paga sempre in contanti, preferisce le consegne il sabato.""",
    "D22": f"""VERBALE DI ASSEMBLEA CONDOMINIALE – {p('mrossi','street')}, Torino
L'amministratore {p('lombardi','full')} apre la seduta alle 21. Presenti: {p('mrossi','full')}, {p('ricci','full')}, {p('deluca','full')} e altri condomini per 640 millesimi.
Punto 1: rifacimento del tetto, preventivo di 48.000 euro. Approvato a maggioranza; vota contro la sig.ra Ricci, che chiede un secondo preventivo.
Punto 2: sostituzione del portone, rinviata alla prossima assemblea.""",
    "D23": f"""VERBALE RIUNIONE DEL PERSONALE – {o('ferramenta','name')}
Presenti: {p('bellini','full')} (titolare), {p('fontana','full')}.
Si decidono i nuovi orari estivi: apertura dalle 8 alle 13 e dalle 15:30 alle 19, chiusura il lunedì mattina da giugno a settembre. Fontana si occuperà dell'inventario di fine luglio.""",
    "D24": f"""CONFERMA APPUNTAMENTO
{o('conti','name')} conferma al sig. {p('moretti','full')} l'appuntamento del 12 novembre 2026 alle ore 10:30 presso lo studio, {o('conti','street')}, Torino, per la pratica di successione. Portare documento d'identità e codice fiscale {p('moretti','cf')}.""",
    "D25": f"""CONFERMA PRENOTAZIONE REVISIONE
{o('officina','name')} conferma alla sig.ra {p('gallo','full')} la revisione periodica dell'auto il 5 dicembre 2026 alle 9:00. Per spostamenti chiamare l'officina; vi ricontatteremo al {p('gallo','phone')}.""",
    "D26": f"""APPUNTAMENTO SOPRALLUOGO
{o('tecno','name')} effettuerà un sopralluogo per la nuova pompa di calore presso la sig.ra {p('colombo','full')}, {p('colombo','street')}, {p('colombo','cap_city')}, giovedì 20 novembre 2026 alle 15. Referente: tel. {p('colombo','phone')}.""",
    "D27": f"""Da: assistenza interna
A: ufficio tecnico
Oggetto: richiamata cliente lavatrice
Ciao, la Ferraris ha richiamato stamattina: il tecnico di {o('sereni','name')} non si è ancora fatto vivo e la lavatrice continua a muoversi in centrifuga. È molto arrabbiata e minaccia di chiedere la sostituzione. Potete darle una data entro venerdì?""",
    "D28": f"""ELENCO PARTECIPANTI – CORSO SICUREZZA SUL LAVORO, 24 OTTOBRE 2026
1. {p('ferraris','full').upper()}
2. {p('bellini','full').upper()}
3. {p('fontana','full').upper()}
4. {p('ricci','full').upper()}
Durata 8 ore, aula 2, docente esterno. Firma di presenza obbligatoria in entrata e in uscita.""",
    "D29": f"""Giorgio, il sig. Esposito mi chiede se si può anticipare la firma della scrittura privata del box a lunedì prossimo, perché il sig. De Luca vorrebbe entrare già dal primo del mese. Fammi sapere se per te va bene.""",
    "D30": f"""PREVENTIVO n. 2026/P-221
{o('tecno','name')} – P.IVA {o('tecno','piva')}
Cliente: {p('esposito','full')}, {p('esposito','street')}, {p('esposito','cap_city')}
Rifacimento completo del bagno: demolizione, nuovi impianti idraulici, piastrelle, sanitari sospesi e box doccia.
Totale preventivato: 6.800,00 euro IVA esclusa. Validità 60 giorni.""",
    "D31": f"""PREVENTIVO n. 2026/P-044
{o('ferramenta','name')} – P.IVA {o('ferramenta','piva')}
Cliente: {o('immobiliare','name')}, {o('immobiliare','street')}, Torino, alla c.a. di {p('lombardi','full')}
Fornitura di 24 serrature di sicurezza a cilindro europeo con montaggio per gli appartamenti in gestione.
Totale: 1.250,00 euro IVA inclusa.""",
    "D32": f"""PREVENTIVO
{o('officina','name')} – P.IVA {o('officina','piva')}
Cliente: {p('dangelo','full')}, tel. {p('dangelo','phone')}
Quattro pneumatici invernali 205/55 R16 con montaggio, equilibratura e deposito delle gomme estive.
Totale: 520,00 euro.""",
    "D33": f"""Gentile cliente {p('barbieri','full')},
la informiamo che a seguito della migrazione dei sistemi della Banca Alpina di Credito le coordinate del suo conto corrente cambieranno dal 1° dicembre 2026.
Vecchio IBAN: {p('barbieri','iban')}
Nuovo IBAN: {NEW_IBAN_BARBIERI}
Le domiciliazioni saranno trasferite automaticamente; la preghiamo di comunicare il nuovo IBAN al suo datore di lavoro.""",
}


def entities():
    """The ground truth of what is personal: every value the documents carry, by kind."""
    out = []
    for x in P.values():
        out += [("PERSON", x["full"]), ("GIVEN", x["given"]), ("SURNAME", x["surname"]), ("EMAIL", x["email"]),
                ("PHONE", x["phone"]), ("STREET", x["street"]), ("CF", x["cf"]), ("IBAN", x["iban"])]
    for x in O.values():
        out += [("ORG", x["name"]), ("EMAIL", x["email"]), ("PIVA", x["piva"]), ("IBAN", x["iban"]), ("STREET", x["street"])]
    out += [("IBAN", NEW_IBAN_BARBIERI), ("ORG", "Banca Alpina di Credito")]
    female = {x["given"]: x["female"] for x in P.values()}
    seen, uniq = set(), []
    for k, v in out:
        if (k, v) not in seen:
            seen.add((k, v))
            uniq.append({"kind": k, "value": v, **({"female": female[v]} if k == "GIVEN" else {})})
    return uniq


Q = [
    # N: the name decides among documents of the same template
    ("N", "Di cosa si lamentava Giulia Ferraris nella sua email?", ["D01"]),
    ("N", "Perché Mario Rossi ha scritto al servizio clienti?", ["D02"]),
    ("N", "Cosa chiede Francesca Gallo nel suo reclamo?", ["D03"]),
    ("N", "Qual è il problema segnalato da Sara Ricci?", ["D04"]),
    ("N", "Quanto ha speso Luca Fontana alla ferramenta?", ["D05"]),
    ("N", "Quanto ha pagato Giulia Ferraris per la manutenzione della caldaia?", ["D06"]),
    ("N", "Quanto costa il tagliando di Elena Barbieri?", ["D07"]),
    ("N", "Che cosa ha comprato Chiara D'Angelo in ferramenta?", ["D08"]),
    ("N", "Qual è il canone d'affitto di Marta Rossi?", ["D09"]),
    ("N", "Quanto paga Nicolò De Luca per il box?", ["D10"]),
    ("N", "Quanto paga Alessandro Moretti di affitto per il negozio?", ["D11"]),
    ("N", "Quando inizia a lavorare Francesca Gallo?", ["D12"]),
    ("N", "Che mansione avrà Luca Fontana?", ["D13"]),
    ("N", "Qual è l'ultimo giorno di lavoro di Sara Ricci?", ["D14"]),
    ("N", "Quanto deve essere rimborsato a Nicolò De Luca?", ["D15"]),
    ("N", "Per cosa chiede il rimborso Elena Barbieri?", ["D16"]),
    ("N", "Quanto ha speso Giorgio Lombardi per il pranzo con i clienti?", ["D17"]),
    ("N", "Quando preferisce essere contattata Giulia Ferraris?", ["D18"]),
    ("N", "Cosa ha chiesto Roberto Esposito come cliente?", ["D19", "D30"]),
    ("N", "A cosa è interessata Chiara D'Angelo?", ["D20"]),
    ("N", "Da quando Mario Rossi è cliente?", ["D21"]),
    ("N", "Per quando è l'appuntamento di Alessandro Moretti allo studio legale?", ["D24"]),
    ("N", "Quando deve portare l'auto in revisione Francesca Gallo?", ["D25"]),
    ("N", "Quando è il sopralluogo da Maria Grazia Colombo?", ["D26"]),
    ("N", "Quanto costa il preventivo del bagno per Roberto Esposito?", ["D30"]),
    ("N", "Quanto costano le gomme invernali per Chiara D'Angelo?", ["D32"]),
    ("N", "Cosa comunica la banca a Elena Barbieri?", ["D33"]),
    # A: an attribute asked by name
    ("A", "Qual è il numero di cellulare di Roberto Esposito?", ["D19"]),
    ("A", "Qual è l'email di Chiara D'Angelo?", ["D20"]),
    ("A", "Dove abita Giulia Ferraris?", ["D18", "D01", "D06"]),
    ("A", "Qual è il codice fiscale di Francesca Gallo?", ["D12"]),
    ("A", "Su quale IBAN va rimborsato Nicolò De Luca?", ["D15"]),
    ("A", "Qual è il nuovo IBAN di Elena Barbieri?", ["D33"]),
    ("A", "Qual è l'indirizzo email di Mario Rossi?", ["D21"]),
    # V: the personal value itself is in the question
    ("V", f"Di chi è il numero {P['esposito']['phone']}?", ["D19"]),
    ("V", f"A chi appartiene l'indirizzo {P['dangelo']['email']}?", ["D20"]),
    ("V", f"Chi abita in {P['ferraris']['street']}?", ["D18", "D01", "D06"]),
    ("V", f"Di chi è il codice fiscale {P['gallo']['cf']}?", ["D12"]),
    ("V", f"Chi ha l'IBAN {P['deluca']['iban']}?", ["D15"]),
    ("V", f"Quale azienda ha la partita IVA {O['ferramenta']['piva']}?", ["D05", "D08", "D31"]),
    # P: only part of the name
    ("P", "Cosa ha detto la Ferraris quando ha richiamato?", ["D27"]),
    ("P", "La fattura della ferramenta per la D'Angelo", ["D08"]),
    ("P", "Il contratto del box del sig. Esposito", ["D10", "D29"]),
    ("P", "Il verbale in cui la Ricci vota contro", ["D22"]),
    ("P", "Quanto costa il preventivo delle serrature per Lombardi?", ["D31"]),
    ("P", "Il rimborso della trasferta di De Luca", ["D15"]),
    ("P", "Il preventivo di Moretti per le gomme", ["D32"]),
    # C: a different case from the document's
    ("C", "giulia ferraris lavatrice rumorosa", ["D01", "D27"]),
    ("C", "elenco partecipanti corso sicurezza FERRARIS", ["D28"]),
    ("C", "corso sulla sicurezza con Luca Fontana", ["D28"]),
    ("C", "marta rossi affitto appartamento cuneo", ["D09"]),
    # O: the company is the key
    ("O", "Cosa ha fatturato Officina Moretti a Elena Barbieri?", ["D07"]),
    ("O", "Che problema ha avuto la cliente con Elettrodomestici Sereni?", ["D01", "D27"]),
    ("O", "Quali serrature fornisce la Ferramenta Bellini all'immobiliare?", ["D31"]),
    ("O", "Assunzione presso lo Studio Legale Conti & Associati", ["D12"]),
    ("O", "Arredamenti Valtellina ha consegnato in ritardo", ["D02"]),
    # S: no personal data in the question
    ("S", "Quale contratto di affitto prevede un canone di 750 euro al mese?", ["D09"]),
    ("S", "Chi ha votato contro il rifacimento del tetto in condominio?", ["D22"]),
    ("S", "C'è un preventivo per rifare il bagno?", ["D30"]),
    ("S", "Quali sono i nuovi orari estivi del negozio di ferramenta?", ["D23"]),
    ("S", "Una richiesta di rimborso per un corso di formazione", ["D16"]),
    ("S", "Lettera di dimissioni o cessazione del rapporto di lavoro", ["D14"]),
    ("S", "Fattura addebitata due volte sulla carta", ["D03"]),
]


def main():
    data = {"docs": [{"id": k, "text": v} for k, v in DOCS.items()], "entities": entities(),
            "questions": [{"cls": c, "q": q, "truth": t} for c, q, t in Q]}
    json.dump(data, open(os.path.join(HERE, "corpus.json"), "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    by = {}
    for x in data["questions"]:
        by[x["cls"]] = by.get(x["cls"], 0) + 1
    print(len(data["docs"]), "docs,", len(data["entities"]), "entities,", len(data["questions"]), "questions", by)


if __name__ == "__main__":
    main()
