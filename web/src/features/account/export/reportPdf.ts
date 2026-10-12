import { jsPDF } from "jspdf";
import { autoTable, type CellInput } from "jspdf-autotable";
import type { TFunction } from "i18next";
import { fetchAccountReportData } from "../api/accountReportData";
import {
  buildAccountReport,
  formatReportCell,
  type AccountReport,
} from "../model/accountReport";

/**
 * The PDF writer. Loaded with `import()` only when the learner presses the
 * button, so jsPDF never reaches the settings chunk or the initial download.
 *
 * jsPDF's built-in fonts are WinAnsi-only and drop every Vietnamese letter
 * outside Latin-1 ("ế", "ộ", "đ", "ư" ...). The report therefore embeds Inter:
 * static Regular and Bold instances cut from the app's own `inter-var.woff2`
 * and subset to Latin + Vietnamese (≈80 kB each), served from
 * `public/fonts/report/` and fetched only here.
 */
export interface ReportFonts {
  /** Base64 TrueType data. */
  regular: string;
  bold: string;
}

const FONT = "Inter";
const MARGIN = 40;
const BRAND: [number, number, number] = [37, 99, 235];
const INK: [number, number, number] = [31, 42, 68];
const MUTED: [number, number, number] = [91, 100, 120];
const HEAD_FILL: [number, number, number] = [232, 238, 252];
const ZEBRA: [number, number, number] = [247, 249, 253];

function toBase64(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  const chunk = 0x8000;
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunk));
  }
  return btoa(binary);
}

async function fetchFont(file: string): Promise<string> {
  const response = await fetch(
    `${import.meta.env.BASE_URL}fonts/report/${file}`,
  );
  if (!response.ok) {
    throw new Error(`Report font ${file} failed to load (${response.status})`);
  }
  return toBase64(await response.arrayBuffer());
}

export async function loadReportFonts(): Promise<ReportFonts> {
  const [regular, bold] = await Promise.all([
    fetchFont("Inter-Regular.ttf"),
    fetchFont("Inter-Bold.ttf"),
  ]);
  return { regular, bold };
}

/** Builds the document; split from saving so it can be checked offline. */
export function createReportPdf(
  report: AccountReport,
  fonts: ReportFonts,
): jsPDF {
  const doc = new jsPDF({ unit: "pt", format: "a4", compress: true });
  doc.addFileToVFS("Inter-Regular.ttf", fonts.regular);
  doc.addFont("Inter-Regular.ttf", FONT, "normal");
  doc.addFileToVFS("Inter-Bold.ttf", fonts.bold);
  doc.addFont("Inter-Bold.ttf", FONT, "bold");
  doc.setFont(FONT, "normal");
  doc.setProperties({ title: report.title, creator: "Fluentra" });

  const pageWidth = doc.internal.pageSize.getWidth();
  const pageHeight = doc.internal.pageSize.getHeight();
  const contentWidth = pageWidth - MARGIN * 2;
  let y = MARGIN;

  const ensureSpace = (needed: number) => {
    if (y + needed > pageHeight - MARGIN) {
      doc.addPage();
      y = MARGIN;
    }
  };

  // Title block
  doc.setFillColor(...BRAND);
  doc.rect(MARGIN, y, 4, 34, "F");
  doc.setFont(FONT, "bold");
  doc.setFontSize(18);
  doc.setTextColor(...INK);
  doc.text(report.title, MARGIN + 14, y + 14);
  doc.setFont(FONT, "normal");
  doc.setFontSize(10);
  doc.setTextColor(...MUTED);
  doc.text(report.generatedLabel, MARGIN + 14, y + 30);
  y += 54;

  // Keeps a heading with at least the table head and a first row.
  const sectionHeading = (text: string) => {
    ensureSpace(72);
    doc.setFont(FONT, "bold");
    doc.setFontSize(13);
    doc.setTextColor(...INK);
    doc.text(text, MARGIN, y);
    y += 8;
  };

  const tableBase = {
    margin: { left: MARGIN, right: MARGIN, top: MARGIN, bottom: MARGIN + 10 },
    styles: {
      font: FONT,
      fontStyle: "normal" as const,
      fontSize: 9,
      textColor: INK,
      cellPadding: 5,
      lineColor: [214, 220, 232] as [number, number, number],
      lineWidth: 0.5,
      overflow: "linebreak" as const,
    },
    headStyles: {
      font: FONT,
      fontStyle: "bold" as const,
      fillColor: HEAD_FILL,
      textColor: INK,
    },
    alternateRowStyles: { fillColor: ZEBRA },
    // A wrapped row stays on one page instead of splitting mid-line.
    rowPageBreak: "avoid" as const,
    didDrawPage: (data: { cursor: { y: number } | null }) => {
      if (data.cursor) y = data.cursor.y;
    },
  };

  // Summary: one compact two-column table per group.
  sectionHeading(report.summaryTitle);
  for (const group of report.summary) {
    autoTable(doc, {
      ...tableBase,
      startY: y + 6,
      theme: "grid",
      tableWidth: contentWidth,
      head: [[{ content: group.title, colSpan: 2 }]],
      body: group.rows.map((row) => [
        row.label,
        formatReportCell(row.value, row.kind, report.locale),
      ]),
      columnStyles: {
        0: { cellWidth: contentWidth * 0.55, textColor: MUTED },
        1: { fontStyle: "bold" },
      },
      // A group is short: move it to the next page rather than split it.
      pageBreak: "avoid",
    });
    y += 10;
  }

  if (report.notes.length > 0) {
    doc.setFont(FONT, "normal");
    doc.setFontSize(9);
    doc.setTextColor(138, 90, 0);
    for (const note of report.notes) {
      const lines = doc.splitTextToSize(note, contentWidth) as string[];
      ensureSpace(lines.length * 12 + 4);
      doc.text(lines, MARGIN, y + 4);
      y += lines.length * 12 + 4;
    }
    y += 6;
  }

  // Detail tables, each starting with a heading.
  for (const table of report.tables) {
    // Room between the previous table's last row and this heading's cap line.
    y += 30;
    sectionHeading(table.title);
    const body: CellInput[][] =
      table.rows.length > 0
        ? table.rows.map((row) =>
            row.map((value, i) =>
              formatReportCell(
                value,
                table.columns[i]?.kind ?? "text",
                report.locale,
              ),
            ),
          )
        : [
            [
              {
                content: report.emptyText,
                colSpan: table.columns.length,
                styles: { textColor: MUTED },
              },
            ],
          ];
    autoTable(doc, {
      ...tableBase,
      startY: y + 6,
      theme: "striped",
      head: [table.columns.map((c) => c.header)],
      body,
      columnStyles: Object.fromEntries(
        table.columns.map((c, i) => [
          i,
          c.kind === "text" ? {} : { halign: "right" as const },
        ]),
      ),
      styles: {
        ...tableBase.styles,
        fontSize: table.columns.length > 7 ? 7.5 : 9,
      },
    });
  }

  // Footer on every page: report name and page x / n.
  const pages = doc.getNumberOfPages();
  for (let page = 1; page <= pages; page += 1) {
    doc.setPage(page);
    doc.setFont(FONT, "normal");
    doc.setFontSize(8);
    doc.setTextColor(...MUTED);
    doc.text(report.title, MARGIN, pageHeight - 20);
    doc.text(`${page} / ${pages}`, pageWidth - MARGIN, pageHeight - 20, {
      align: "right",
    });
  }
  return doc;
}

export async function downloadReportPdf(report: AccountReport): Promise<void> {
  const fonts = await loadReportFonts();
  createReportPdf(report, fonts).save(`${report.fileBaseName}.pdf`);
}

/** Reads the learner's data, builds the report and saves the PDF. */
export async function exportAccountPdf(
  t: TFunction,
  locale: string,
): Promise<void> {
  const [data, fonts] = await Promise.all([
    fetchAccountReportData(),
    loadReportFonts(),
  ]);
  const report = buildAccountReport(data, t, locale);
  createReportPdf(report, fonts).save(`${report.fileBaseName}.pdf`);
}
