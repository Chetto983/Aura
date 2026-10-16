"""Build the complex retrieval corpus and its questions, with the gold known by construction.

    python3 spikes/document-routing-benchmark/generate_complex_corpus.py [--out CORPUS_DIR]

Writes the PDFs and workbooks to CORPUS_DIR (default scripts/fixtures/document_retrieval_eval/
complex_corpus), has LibreOffice recalculate and re-save every
workbook (formulas then carry cached values, as Excel's do), and writes the questions to
complex_qrels.json and the checksums to complex_corpus.sha256 beside it, outside the corpus so
neither is ingested as a document. The workbooks are normalized as generate_corpus.py's are, so
a re-run reproduces every byte.
Look-alikes are deliberate: two packaging machines with the same alarm table layout and the
2025 budget beside the 2026 one, so a question is answered only by telling them apart.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import pathlib
import shutil
import subprocess
import tempfile

import complex_corpus_pdf as pdf
import complex_corpus_xlsx as xlsx
from generate_corpus import normalize_xlsx

SCHEMA = "aura.document-retrieval-eval/v1"
LIBREOFFICE_IMAGE = "aura-ingest:bench"


def _recalculated(workbooks: dict[str, object], out: pathlib.Path) -> None:
    """Save each workbook, then let LibreOffice recalculate and re-save it in place."""
    with tempfile.TemporaryDirectory() as raw:
        for name, workbook in workbooks.items():
            workbook.save(pathlib.Path(raw, name))
        converted = pathlib.Path(raw, "converted")
        converted.mkdir()
        subprocess.run(
            [
                "docker",
                "run",
                "--rm",
                "-v",
                f"{raw}:/w",
                "--entrypoint",
                "soffice",
                LIBREOFFICE_IMAGE,
                "--headless",
                "--calc",
                "--convert-to",
                "xlsx:Calc MS Excel 2007 XML",
                "--outdir",
                "/w/converted",
                *[f"/w/{name}" for name in workbooks],
            ],
            check=True,
            capture_output=True,
        )
        for name in workbooks:
            shutil.copyfile(converted / name, out / name)
            normalize_xlsx(str(out / name))


def ask(qid: str, category: str, gold: list[str], query: str, answer: str) -> dict:
    return {"qid": qid, "category": category, "gold_docs": gold, "query": query, "answer": answer}


def _alarm_questions(fx300: dict, fx200: dict) -> list[dict]:
    differing = [
        code for code in fx300["alarms"] if fx300["alarms"][code]["cause"] != fx200["alarms"][code]["cause"]
    ]
    picks = [differing[3], differing[17], differing[-2]]
    questions = [
        ask(
            f"fx300-alarm-{code}",
            "TABLE_DEEP",
            ["manuale_confezionatrice_FX300.pdf"],
            f"sulla confezionatrice FX300 è comparso l'allarme {code}: qual è la causa probabile?",
            fx300["alarms"][code]["cause"],
        )
        for code in picks
    ]
    code = differing[9]
    questions.append(
        ask(
            f"fx200-alarm-{code}",
            "LOOKALIKE",
            ["manuale_confezionatrice_FX200.pdf"],
            f"allarme {code} sulla FX200, come si risolve?",
            fx200["alarms"][code]["remedy"],
        )
    )
    return questions


def _questions(f: dict) -> list[dict]:
    fx300, fx200, contract, report, audit, sds = (
        f["fx300"],
        f["fx200"],
        f["contract"],
        f["report"],
        f["audit"],
        f["sds"],
    )
    b26, b25, parts, plan, rota = f["budget2026"], f["budget2025"], f["parts"], f["plan"], f["rota"]
    chosen = list(parts["chosen"].items())
    rc = sorted(report["complaints"])[9]
    nc = "NC-03"
    pz8 = "Pallettizzatore PZ8"
    q = _alarm_questions(fx300, fx200)
    q += [
        ask(
            "fx300-speed",
            "PROSE",
            ["manuale_confezionatrice_FX300.pdf"],
            "quanti pezzi al minuto può confezionare la FX300",
            str(fx300["speed"]),
        ),
        ask(
            "fx300-chains",
            "TABLE",
            ["manuale_confezionatrice_FX300.pdf"],
            "ogni quante ore vanno lubrificate le catene della confezionatrice FX300",
            str(fx300["intervals"]["Lubrificazione catene"]),
        ),
        ask(
            "fx300-blade-part",
            "CROSS_DOC",
            ["manuale_confezionatrice_FX300.pdf", "magazzino_ricambi_2026.xlsx"],
            "codice ricambio della lama di taglio seghettata della FX300",
            fx300["blade_part"],
        ),
        ask(
            "fx300-parameter",
            "TABLE_DEEP",
            ["manuale_confezionatrice_FX300.pdf"],
            "valore di fabbrica del parametro P067 sul pannello della FX300",
            str(fx300["parameters"]["P067"][1]),
        ),
        ask(
            "fx200-torque-m10",
            "LOOKALIKE",
            ["manuale_confezionatrice_FX200.pdf"],
            "coppia di serraggio delle viti M10 sulla FX200",
            str(fx200["torque"]["M10"]),
        ),
        ask(
            "contract-penalty",
            "PROSE",
            ["capitolato_fornitura_linea3.pdf"],
            "penale giornaliera per ritardo nella consegna della Linea 3",
            f"{contract['penalty']}%",
        ),
        ask(
            "contract-penalty-paraphrase",
            "PARAPHRASE",
            ["capitolato_fornitura_linea3.pdf"],
            "quanto ci deve il fornitore per ogni giorno in cui arriva tardi con la nuova linea",
            f"{contract['penalty']}%",
        ),
        ask(
            "contract-warranty",
            "PROSE",
            ["capitolato_fornitura_linea3.pdf"],
            "quanti mesi di garanzia dopo il collaudo in sito",
            str(contract["warranty"]),
        ),
        ask(
            "contract-advance",
            "PROSE",
            ["capitolato_fornitura_linea3.pdf"],
            "che percentuale si paga all'ordine della linea di confezionamento",
            f"{contract['split'][0]}%",
        ),
        ask(
            "report-oee",
            "TABLE",
            ["report_qualita_Q3_2026.pdf"],
            "OEE della Linea 2 nel mese di agosto",
            f"{report['oee']['Linea 2'][1]:.1%}",
        ),
        ask(
            "report-complaint",
            "EXACT",
            ["report_qualita_Q3_2026.pdf"],
            f"che difetto riguardava il reclamo {rc}",
            report["complaints"][rc][0],
        ),
        ask(
            "report-best-line",
            "AGG",
            ["report_qualita_Q3_2026.pdf"],
            "quale linea ha avuto l'efficienza migliore nel terzo trimestre",
            report["best_line"],
        ),
        ask(
            "audit-nc",
            "EXACT",
            ["verbale_audit_ISO9001_2026.pdf"],
            f"a quale requisito della norma si riferisce la non conformità {nc}",
            audit["findings"][nc]["clause"],
        ),
        ask(
            "audit-majors",
            "AGG",
            ["verbale_audit_ISO9001_2026.pdf"],
            "quante non conformità maggiori sono emerse all'audit di certificazione",
            str(audit["majors"]),
        ),
        ask(
            "audit-auditor",
            "PROSE",
            ["verbale_audit_ISO9001_2026.pdf"],
            "chi era l'auditor responsabile della verifica ISO 9001",
            "ing. Paola Ferrero",
        ),
        ask(
            "sds-flash",
            "PROSE",
            ["scheda_sicurezza_detergente_DX45.pdf"],
            "punto di infiammabilità del detergente DX45",
            f"{sds['flash']} °C",
        ),
        ask(
            "sds-gloves",
            "PARAPHRASE",
            ["scheda_sicurezza_detergente_DX45.pdf"],
            "che protezioni mettere alle mani quando si usa il detergente alcalino",
            "guanti in nitrile",
        ),
        ask(
            "sds-un",
            "EXACT",
            ["scheda_sicurezza_detergente_DX45.pdf"],
            "numero ONU per spedire il DX45",
            "UN 1719",
        ),
        ask(
            "budget-centro-2026",
            "LOOKALIKE",
            ["budget_consolidato_2026.xlsx"],
            "ricavi 2026 dell'area Centro",
            str(b26["region_totals"]["Centro"]),
        ),
        ask(
            "budget-inflation-2025",
            "LOOKALIKE",
            ["budget_consolidato_2025.xlsx"],
            "ipotesi di inflazione sui costi usata nel budget 2025",
            str(b25["inflation"]),
        ),
        ask(
            "budget-top-centre",
            "AGG",
            ["budget_consolidato_2026.xlsx"],
            "quale centro di costo spende di più nel budget 2026",
            b26["top_centre"],
        ),
        ask(
            "budget-ebitda",
            "AGG",
            ["budget_consolidato_2026.xlsx"],
            "EBITDA previsto per il 2026",
            str(b26["ebitda"]),
        ),
        ask(
            "parts-location",
            "EXACT",
            ["magazzino_ricambi_2026.xlsx"],
            f"dove è ubicato il ricambio {chosen[0][0]}",
            chosen[0][1]["location"],
        ),
        ask(
            "parts-supplier",
            "EXACT",
            ["magazzino_ricambi_2026.xlsx"],
            f"chi fornisce l'articolo {chosen[1][0]}",
            chosen[1][1]["supplier"],
        ),
        ask(
            "parts-lead-time",
            "AGG",
            ["magazzino_ricambi_2026.xlsx"],
            "quale fornitore di ricambi ha i tempi di consegna più lunghi",
            parts["slowest_supplier"],
        ),
        ask(
            "parts-below-min",
            "AGG",
            ["magazzino_ricambi_2026.xlsx"],
            "quanti articoli di magazzino sono sotto la scorta minima",
            str(parts["below_minimum"]),
        ),
        ask(
            "plan-pz8",
            "MATRIX",
            ["piano_manutenzione_2026.xlsx"],
            f"quante revisioni generali sono pianificate per il {pz8} nel 2026",
            str(plan["revisions"].get(pz8, 0)),
        ),
        ask(
            "plan-legend",
            "MATRIX",
            ["piano_manutenzione_2026.xlsx"],
            "cosa vuol dire la lettera T nel piano di manutenzione",
            "Taratura sensori",
        ),
        ask(
            "rota-nights",
            "MATRIX",
            ["turni_reparto_ottobre_2026.xlsx"],
            "chi ha fatto più turni di notte in reparto a ottobre",
            rota["most_nights"],
        ),
        ask(
            "rota-legend",
            "MATRIX",
            ["turni_reparto_ottobre_2026.xlsx"],
            "a che ora inizia il turno di notte secondo il foglio turni",
            "22",
        ),
    ]
    return q


def build(out: pathlib.Path) -> list[dict]:
    out.mkdir(parents=True, exist_ok=True)
    facts = {
        "fx300": pdf.machine_manual(str(out / "manuale_confezionatrice_FX300.pdf"), "FX300", 4, seed=300),
        "fx200": pdf.machine_manual(str(out / "manuale_confezionatrice_FX200.pdf"), "FX200", 7, seed=200),
        "contract": pdf.supply_contract(str(out / "capitolato_fornitura_linea3.pdf"), seed=3),
        "report": pdf.quality_report(str(out / "report_qualita_Q3_2026.pdf"), seed=33),
        "audit": pdf.audit_report(str(out / "verbale_audit_ISO9001_2026.pdf"), seed=9001),
        "sds": pdf.safety_sheet(str(out / "scheda_sicurezza_detergente_DX45.pdf"), seed=45),
    }
    workbooks = {}
    for name, key, (workbook, fact) in (
        ("budget_consolidato_2026.xlsx", "budget2026", xlsx.budget(2026, seed=2026)),
        ("budget_consolidato_2025.xlsx", "budget2025", xlsx.budget(2025, seed=2025)),
        ("magazzino_ricambi_2026.xlsx", "parts", xlsx.spare_parts(seed=7)),
        ("piano_manutenzione_2026.xlsx", "plan", xlsx.maintenance_plan(seed=12)),
        ("turni_reparto_ottobre_2026.xlsx", "rota", xlsx.shifts(seed=10)),
    ):
        workbooks[name] = workbook
        facts[key] = fact
    _recalculated(workbooks, out)
    questions = _questions(facts)
    (out.parent / "complex_qrels.json").write_text(
        json.dumps({"schema_id": SCHEMA, "questions": questions}, ensure_ascii=False, indent=1) + "\n"
    )
    return questions


def write_checksums(out: pathlib.Path) -> None:
    lines = [
        f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {out.name}/{path.name}\n"
        for path in sorted(out.iterdir())
    ]
    (out.parent / f"{out.name}.sha256").write_text("".join(lines), encoding="ascii")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    default = (
        pathlib.Path(__file__).resolve().parents[2]
        / "scripts/fixtures/document_retrieval_eval/complex_corpus"
    )
    parser.add_argument("--out", type=pathlib.Path, default=default, help="corpus output directory")
    corpus = parser.parse_args().out.resolve()
    questions = build(corpus)
    write_checksums(corpus)
    print(f"generated {len(list(corpus.iterdir()))} files and {len(questions)} questions in {corpus}")
