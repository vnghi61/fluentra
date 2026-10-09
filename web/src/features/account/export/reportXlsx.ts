import writeXlsxFile, {
  type Cell,
  type CellObject,
  type Row,
  type Sheet,
} from "write-excel-file/browser";
import type { TFunction } from "i18next";
import { fetchAccountReportData } from "../api/accountReportData";
import {
  buildAccountReport,
  type AccountReport,
  type ReportCellKind,
  type ReportCellValue,
} from "../model/accountReport";

/**
 * The Excel writer. Loaded with `import()` only when the learner presses the
 * button, so `write-excel-file` (and its zip dependency) never reaches the
 * settings chunk or the initial download.
 */

type WorkbookSheet = Sheet<File | Blob | ArrayBuffer>;

const HEADER_FILL = "#E8EEFC";
const HEADER_TEXT = "#1F2A44";
const BORDER = "#C9D1E3";
const DATE_FORMAT = "dd/mm/yyyy";
const DATETIME_FORMAT = "dd/mm/yyyy hh:mm";

/** Excel sheet names: ≤ 31 characters, none of []:*?/\ and unique. */
function sheetName(title: string, used: Set<string>): string {
  const base =
    title
      .replace(/[[\]:*?/\\]/g, " ")
      .trim()
      .slice(0, 31) || "Sheet";
  let name = base;
  for (let n = 2; used.has(name.toLowerCase()); n += 1) {
    const suffix = ` (${n})`;
    name = `${base.slice(0, 31 - suffix.length)}${suffix}`;
  }
  used.add(name.toLowerCase());
  return name;
}

function cell(value: ReportCellValue, kind: ReportCellKind): Cell {
  if (value === null || value === "") return null;
  if (value instanceof Date) {
    return {
      value,
      type: Date,
      format: kind === "datetime" ? DATETIME_FORMAT : DATE_FORMAT,
      align: "left",
    };
  }
  if (typeof value === "number" && kind === "percent") {
    // A real percentage, so Excel shows "62.5%" and still sums and sorts it.
    return {
      value: value / 100,
      type: Number,
      format: Number.isInteger(value) ? "0%" : "0.0%",
    };
  }
  if (typeof value === "number") {
    return {
      value,
      type: Number,
      format: Number.isInteger(value) ? "0" : "0.0",
    };
  }
  return { value, type: String, wrap: value.length > 60 };
}

function headerCell(text: string): CellObject {
  return {
    value: text,
    type: String,
    fontWeight: "bold",
    textColor: HEADER_TEXT,
    backgroundColor: HEADER_FILL,
    bottomBorderColor: BORDER,
    bottomBorderStyle: "thin",
    alignVertical: "center",
    height: 22,
  };
}

/** A width in characters that fits the longest value, within reason. */
function widthFor(
  values: readonly (string | number | Date | null)[],
  kind: ReportCellKind,
): number {
  if (kind === "datetime") return 18;
  if (kind === "date") return 13;
  let longest = 0;
  for (const value of values) {
    const text = value === null ? "" : String(value);
    longest = Math.max(longest, text.length);
  }
  return Math.min(Math.max(longest + 2, 10), 60);
}

export function reportSheets(report: AccountReport): WorkbookSheet[] {
  const used = new Set<string>();
  const sheets: WorkbookSheet[] = [];

  // 1. Summary: title, date, then one block per group as label / value.
  const summary: Row[] = [
    [
      {
        value: report.title,
        type: String,
        fontWeight: "bold",
        fontSize: 16,
        height: 26,
      },
    ],
    [{ value: report.generatedLabel, type: String, textColor: "#5B6478" }],
    [],
  ];
  const labels: string[] = [];
  const values: string[] = [];
  for (const group of report.summary) {
    summary.push([{ ...headerCell(group.title), columnSpan: 2 }, null]);
    for (const row of group.rows) {
      summary.push([
        { value: row.label, type: String },
        cell(row.value, row.kind),
      ]);
      labels.push(row.label);
      values.push(row.value === null ? "" : String(row.value));
    }
    summary.push([]);
  }
  for (const note of report.notes) {
    summary.push([
      { value: note, type: String, fontStyle: "italic", textColor: "#8A5A00" },
    ]);
  }
  sheets.push({
    sheet: sheetName(report.summaryTitle, used),
    data: summary,
    columns: [
      { width: Math.min(Math.max(...labels.map((l) => l.length), 24) + 2, 48) },
      { width: Math.min(Math.max(...values.map((v) => v.length), 16) + 2, 48) },
    ],
    showGridLines: false,
  });

  // 2. One sheet per table, with a frozen header row.
  for (const table of report.tables) {
    const data: Row[] = [table.columns.map((c) => headerCell(c.header))];
    for (const row of table.rows) {
      data.push(
        row.map((value, i) => cell(value, table.columns[i]?.kind ?? "text")),
      );
    }
    if (table.rows.length === 0) {
      data.push([
        { value: report.emptyText, type: String, fontStyle: "italic" },
      ]);
    }
    sheets.push({
      sheet: sheetName(table.title, used),
      data,
      stickyRowsCount: 1,
      columns: table.columns.map((column, i) => ({
        width: Math.max(
          widthFor(
            table.rows.map((row) => row[i] ?? null),
            column.kind,
          ),
          Math.min(column.header.length + 2, 40),
        ),
      })),
    });
  }
  return sheets;
}

export async function downloadReportXlsx(report: AccountReport): Promise<void> {
  await writeXlsxFile(reportSheets(report), {
    fontFamily: "Calibri",
    fontSize: 11,
  }).toFile(`${report.fileBaseName}.xlsx`);
}

/** Reads the learner's data, builds the report and saves the workbook. */
export async function exportAccountXlsx(
  t: TFunction,
  locale: string,
): Promise<void> {
  const data = await fetchAccountReportData();
  await downloadReportXlsx(buildAccountReport(data, t, locale));
}
