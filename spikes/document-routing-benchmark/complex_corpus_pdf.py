"""PDFs of the complex retrieval corpus: long manuals, contracts, reports and a safety sheet.

Each document is laid out the way the real ones are: running headers and footers, two-column
pages, tables that run over several pages with their header repeated, footnotes, and a
landscape page. Every fact a question asks for is returned so the gold is known, not judged.
"""

from __future__ import annotations

import random

from reportlab import rl_config
from reportlab.lib import colors
from reportlab.lib.pagesizes import A4, landscape
from reportlab.lib.styles import getSampleStyleSheet
from reportlab.lib.units import cm
from reportlab.platypus import (
    BaseDocTemplate,
    Frame,
    NextPageTemplate,
    PageBreak,
    PageTemplate,
    Paragraph,
    Spacer,
    Table,
    TableStyle,
)

rl_config.invariant = 1
STYLES = getSampleStyleSheet()
BODY, H1, H2 = STYLES["BodyText"], STYLES["Heading1"], STYLES["Heading2"]
GRID = TableStyle(
    [
        ("GRID", (0, 0), (-1, -1), 0.4, colors.grey),
        ("FONTSIZE", (0, 0), (-1, -1), 7.5),
        ("BACKGROUND", (0, 0), (-1, 0), colors.lightgrey),
        ("VALIGN", (0, 0), (-1, -1), "TOP"),
    ]
)


def _document(path: str, title: str, footer: str) -> BaseDocTemplate:
    def decorate(canvas, doc):
        canvas.saveState()
        canvas.setFont("Helvetica", 7)
        width, height = doc.pagesize
        canvas.drawString(1.5 * cm, height - 1 * cm, title)
        canvas.drawRightString(width - 1.5 * cm, 1 * cm, f"{footer} — pagina {doc.page}")
        canvas.restoreState()

    doc = BaseDocTemplate(
        path,
        pagesize=A4,
        title=title,
        author="Ufficio Tecnico",
        leftMargin=1.5 * cm,
        rightMargin=1.5 * cm,
        topMargin=1.6 * cm,
        bottomMargin=1.6 * cm,
    )
    width, height = A4
    single = Frame(doc.leftMargin, doc.bottomMargin, doc.width, doc.height, id="one")
    half = (doc.width - 0.6 * cm) / 2
    left = Frame(doc.leftMargin, doc.bottomMargin, half, doc.height, id="left")
    right = Frame(doc.leftMargin + half + 0.6 * cm, doc.bottomMargin, half, doc.height, id="right")
    wide = Frame(1.5 * cm, 1.6 * cm, height - 3 * cm, width - 3.2 * cm, id="wide")
    doc.addPageTemplates(
        [
            PageTemplate(id="single", frames=[single], onPage=decorate),
            PageTemplate(id="columns", frames=[left, right], onPage=decorate),
            PageTemplate(id="landscape", frames=[wide], onPage=decorate, pagesize=landscape(A4)),
        ]
    )
    return doc


def _paragraphs(rng: random.Random, topic: str, count: int) -> list:
    clauses = [
        "l'operatore verifica lo stato dei ripari prima dell'avvio",
        "la pressione dell'aria compressa deve restare tra 6 e 7 bar",
        "ogni intervento è registrato nel registro di manutenzione",
        "i dispositivi di sicurezza non devono essere esclusi",
        "la pulizia si esegue a macchina ferma e sezionata",
        "i ricambi devono essere originali o approvati dal costruttore",
    ]
    return [Paragraph(f"{topic}: " + "; ".join(rng.sample(clauses, 3)) + ".", BODY) for _ in range(count)]


def machine_manual(path: str, model: str, revision: int, seed: int) -> dict:
    """A packaging machine's manual with a 120-row alarm table across pages."""
    rng = random.Random(seed)
    doc = _document(path, f"{model} — Manuale d'uso e manutenzione, rev. {revision}", model)
    story = [
        Paragraph(f"Confezionatrice {model}", H1),
        Paragraph(f"Revisione {revision}", BODY),
        NextPageTemplate("columns"),
        PageBreak(),
        Paragraph("1. Avvertenze generali", H2),
    ]
    story += _paragraphs(rng, "Sicurezza", 10)
    story.append(Paragraph("2. Descrizione della macchina", H2))
    speed = rng.randrange(60, 140)
    story.append(
        Paragraph(
            f"La {model} confeziona fino a {speed} pezzi al minuto con film termoretraibile. "
            "La potenza installata è indicata nella targa dati<super>1</super>.",
            BODY,
        )
    )
    story += _paragraphs(rng, "Funzionamento", 8)
    story += [
        Paragraph("<super>1</super> Il valore di targa prevale su quello del catalogo.", STYLES["Italic"]),
        NextPageTemplate("single"),
        PageBreak(),
        Paragraph("3. Codici di allarme", H2),
    ]
    causes = [
        "fotocellula di ingresso sporca",
        "film esaurito",
        "temperatura saldatura fuori soglia",
        "riparo aperto",
        "sovraccarico motore principale",
        "pressione aria insufficiente",
        "encoder non allineato",
        "inceppamento sul nastro di uscita",
    ]
    remedies = [
        "pulire con panno asciutto",
        "sostituire la bobina",
        "verificare la termocoppia",
        "chiudere il riparo e riarmare",
        "controllare il termico del motore",
        "verificare il regolatore di pressione",
        "rifare la fase di zero",
        "rimuovere il prodotto",
    ]
    rows = [["Codice", "Descrizione", "Causa probabile", "Rimedio"]]
    alarms: dict[str, dict] = {}
    for number in range(1, 121):
        code = f"A{number:03d}"
        pick = rng.randrange(len(causes))
        entry = {"cause": causes[pick], "remedy": remedies[pick]}
        rows.append(
            [
                code,
                f"Arresto {rng.choice(['immediato', 'controllato', 'a fine ciclo'])}",
                entry["cause"],
                entry["remedy"],
            ]
        )
        alarms[code] = entry
    story += [
        Table(rows, repeatRows=1, colWidths=[2 * cm, 4 * cm, 6 * cm, 6 * cm], style=GRID),
        PageBreak(),
        Paragraph("4. Intervalli di manutenzione", H2),
    ]
    intervals = {
        "Lubrificazione catene": rng.choice([40, 80, 160]),
        "Sostituzione lame": rng.choice([250, 500]),
        "Controllo cinghie": rng.choice([160, 320]),
        "Pulizia barre saldanti": rng.choice([8, 24]),
    }
    story.append(
        Table(
            [["Attività", "Ogni (ore)"], *[[k, str(v)] for k, v in intervals.items()]],
            colWidths=[9 * cm, 4 * cm],
            style=GRID,
        )
    )
    torque = {f"M{size}": round(size * rng.uniform(2.1, 2.6), 1) for size in (6, 8, 10, 12)}
    story += [
        Spacer(1, 12),
        Paragraph("5. Coppie di serraggio", H2),
        Table([["Vite", "Coppia (Nm)"], *[[k, str(v)] for k, v in torque.items()]], style=GRID),
    ]
    story += _paragraphs(rng, "Manutenzione", 12)
    parts, blade = _spare_parts_table(rng, model)
    parameters = {
        f"P{n:03d}": (
            rng.choice(["Ritardo", "Soglia", "Velocità", "Tempo", "Offset"])
            + f" {rng.choice(['saldatura', 'taglio', 'nastro', 'film', 'uscita'])}",
            rng.randrange(1, 900),
        )
        for n in range(1, 81)
    }
    story += [
        PageBreak(),
        Paragraph("6. Ricambi consigliati", H2),
        parts,
        PageBreak(),
        Paragraph("7. Parametri del pannello operatore", H2),
        Table(
            [
                ["Parametro", "Funzione", "Valore di fabbrica"],
                *[[k, v[0], str(v[1])] for k, v in parameters.items()],
            ],
            repeatRows=1,
            style=GRID,
        ),
        NextPageTemplate("columns"),
        PageBreak(),
        Paragraph("8. Diagnostica avanzata", H2),
    ]
    story += _paragraphs(rng, "Diagnostica", 16)
    doc.build(story)
    return {
        "speed": speed,
        "alarms": alarms,
        "intervals": intervals,
        "torque": torque,
        "blade_part": blade,
        "parameters": parameters,
    }


def _spare_parts_table(rng: random.Random, model: str) -> tuple[Table, str]:
    """150 recommended spares whose codes are the store's RIC- codes, so a manual and the
    store workbook name the same part."""
    groups = [
        "Gruppo saldatura",
        "Gruppo taglio",
        "Alimentazione film",
        "Nastro di uscita",
        "Quadro elettrico",
    ]
    rows = [["Pos.", "Gruppo", "Descrizione", "Codice ricambio", "Q.tà consigliata"]]
    blade = ""
    for position in range(1, 151):
        code = f"RIC-{10000 + rng.randrange(1500) * 7}"
        group = groups[position % len(groups)]
        description = (
            "Lama di taglio seghettata" if position == 42 else f"Componente {group.lower()} {position}"
        )
        if position == 42:
            blade = code
        rows.append([str(position), group, description, code, str(rng.randrange(1, 5))])
    return Table(rows, repeatRows=1, style=GRID), blade


def supply_contract(path: str, seed: int) -> dict:
    rng = random.Random(seed)
    doc = _document(path, "Capitolato tecnico — Fornitura Linea 3 di confezionamento", "Capitolato L3")
    penalty = rng.choice([0.3, 0.5, 0.8])
    cap = rng.choice([8, 10, 12])
    warranty = rng.choice([18, 24, 36])
    split = rng.choice([(30, 60, 10), (20, 70, 10), (40, 50, 10)])
    articles = {
        "Oggetto": "fornitura, installazione e messa in servizio della Linea 3",
        "Collaudo in fabbrica (FAT)": "si svolge presso il fornitore alla presenza del committente",
        "Collaudo in sito (SAT)": "verifica una produzione continua di 8 ore senza fermi",
        "Penali": f"per ogni giorno di ritardo si applica una penale dello {penalty}% "
        f"fino al {cap}% del valore",
        "Garanzia": f"la garanzia copre {warranty} mesi dalla firma del verbale di SAT",
        "Pagamenti": f"{split[0]}% all'ordine, {split[1]}% alla consegna, {split[2]}% al SAT",
        "Riservatezza": "le informazioni tecniche restano riservate per cinque anni",
    }
    story = [Paragraph("Capitolato tecnico di fornitura", H1)]
    for extra in (
        "Documentazione",
        "Formazione del personale",
        "Ricambi e assistenza",
        "Sicurezza in cantiere",
        "Varianti in corso d'opera",
        "Recesso",
        "Foro competente",
    ):
        articles[extra] = f"si rinvia all'allegato tecnico per {extra.lower()}"
    for number, (title, text) in enumerate(articles.items(), start=1):
        story += [Paragraph(f"Art. {number} — {title}", H2), Paragraph(text.capitalize() + ".", BODY)]
        story += _paragraphs(rng, title, 5)
    story += [
        Paragraph("Allegato A — Tabella penali per milestone", H2),
        Table(
            [
                ["Milestone", "Penale giornaliera", "Massimale"],
                ["Consegna", f"{penalty}%", f"{cap}%"],
                ["FAT", f"{penalty / 2}%", f"{cap / 2}%"],
                ["SAT", f"{penalty}%", f"{cap}%"],
            ],
            style=GRID,
        ),
    ]
    doc.build(story)
    return {"penalty": penalty, "cap": cap, "warranty": warranty, "split": split}


def quality_report(path: str, seed: int) -> dict:
    rng = random.Random(seed)
    doc = _document(path, "Report qualità — terzo trimestre 2026", "Report Q3 2026")
    lines = ["Linea 1", "Linea 2", "Linea 3", "Linea 4"]
    months = ["Luglio", "Agosto", "Settembre"]
    oee = {line: [round(rng.uniform(0.62, 0.88), 3) for _ in months] for line in lines}
    complaints = [
        (
            f"RC-2026-{n:03d}",
            rng.choice(
                ["etichetta illeggibile", "film strappato", "quantità errata", "prodotto danneggiato"]
            ),
            rng.choice(lines),
        )
        for n in range(31, 49)
    ]
    story = [
        Paragraph("Report qualità Q3 2026", H1),
        Paragraph("Sintesi per la direzione.", BODY),
        NextPageTemplate("landscape"),
        PageBreak(),
        Paragraph("OEE per linea e mese", H2),
        Table(
            [["Linea", *months], *[[line, *[f"{v:.1%}" for v in values]] for line, values in oee.items()]],
            style=GRID,
        ),
        Spacer(1, 12),
        Paragraph("Reclami clienti", H2),
        Table([["Numero", "Difetto", "Linea"], *[list(c) for c in complaints]], repeatRows=1, style=GRID),
        NextPageTemplate("single"),
        PageBreak(),
        Paragraph("Azioni correttive", H2),
    ]
    story += _paragraphs(rng, "Azione", 6)
    doc.build(story)
    best = max(lines, key=lambda line: sum(oee[line]) / 3)
    return {"oee": oee, "best_line": best, "complaints": {c[0]: (c[1], c[2]) for c in complaints}}


def audit_report(path: str, seed: int) -> dict:
    rng = random.Random(seed)
    doc = _document(path, "Verbale di audit ISO 9001:2015 — 2026", "Audit ISO 9001")
    findings = []
    for number in range(1, 8):
        clause = rng.choice(["7.1.5", "7.2", "8.4", "8.5.1", "9.1.2", "10.2"])
        grade = rng.choice(["maggiore", "minore", "minore"])
        days = 30 if grade == "maggiore" else 90
        findings.append(
            (
                f"NC-{number:02d}",
                clause,
                grade,
                rng.choice(
                    [
                        "taratura strumenti scaduta",
                        "formazione non registrata",
                        "valutazione fornitori incompleta",
                        "riesame non documentato",
                    ]
                ),
                days,
            )
        )
    story = [
        Paragraph("Verbale di audit di certificazione", H1),
        Paragraph("Ente: Certiquality — Auditor responsabile: ing. Paola Ferrero.", BODY),
        Table(
            [
                ["N.", "Requisito", "Gravità", "Rilievo", "Chiusura (giorni)"],
                *[[f[0], f[1], f[2], f[3], str(f[4])] for f in findings],
            ],
            repeatRows=1,
            style=GRID,
        ),
    ]
    story += [
        Paragraph("Campo di applicazione e processi verificati", H2),
        Table(
            [
                ["Processo", "Responsabile", "Esito"],
                *[
                    [
                        p,
                        rng.choice(["R. Gallo", "M. Conti", "S. Greco"]),
                        rng.choice(["conforme", "con rilievi"]),
                    ]
                    for p in (
                        "Progettazione",
                        "Acquisti",
                        "Produzione",
                        "Controllo qualità",
                        "Magazzino",
                        "Manutenzione",
                        "Formazione",
                        "Gestione reclami",
                    )
                ],
            ],
            style=GRID,
        ),
        Paragraph("Osservazioni e opportunità di miglioramento", H2),
    ]
    story += _paragraphs(rng, "Osservazione", 14)
    doc.build(story)
    return {
        "findings": {f[0]: {"clause": f[1], "grade": f[2], "issue": f[3]} for f in findings},
        "majors": sum(1 for f in findings if f[2] == "maggiore"),
    }


def safety_sheet(path: str, seed: int) -> dict:
    rng = random.Random(seed)
    doc = _document(path, "Scheda di dati di sicurezza — Detergente alcalino DX45", "SDS DX45")
    flash = rng.choice([61, 72, 85])
    sections = [
        "Identificazione",
        "Identificazione dei pericoli",
        "Composizione",
        "Primo soccorso",
        "Misure antincendio",
        "Rilascio accidentale",
        "Manipolazione e immagazzinamento",
        "Controllo dell'esposizione e protezione individuale",
        "Proprietà fisiche e chimiche",
        "Stabilità e reattività",
        "Informazioni tossicologiche",
        "Informazioni ecologiche",
        "Smaltimento",
        "Trasporto",
        "Regolamentazione",
        "Altre informazioni",
    ]
    texts = {
        2: "H314 Provoca gravi ustioni cutanee e gravi lesioni oculari. "
        "H290 Può essere corrosivo per i metalli.",
        8: "Indossare guanti in nitrile, occhiali a tenuta e grembiule antiacido.",
        9: f"Punto di infiammabilità: {flash} °C. pH (soluzione 1%): 12,5.",
        14: "UN 1719, classe 8, gruppo di imballaggio II.",
    }
    story = [Paragraph("Scheda di dati di sicurezza DX45", H1)]
    for number, title in enumerate(sections, start=1):
        story += [
            Paragraph(f"Sezione {number}: {title}", H2),
            Paragraph(texts.get(number, "Nessuna informazione specifica oltre a quanto indicato."), BODY),
        ]
    doc.build(story)
    return {"flash": flash}
