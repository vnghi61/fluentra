import { describe, expect, it } from "vitest";

import {
  countWords,
  importWordFile,
  splitAtLimit,
} from "@/features/vocabulary/model/wordList";

describe("importWordFile", () => {
  it("reads an Anki plain-text export: headers, HTML and sound tags", () => {
    const anki = [
      "#separator:tab",
      "#html:true",
      "#tags column:3",
      "leisure\tthời gian <b>rảnh</b>&nbsp;[sound:leisure.mp3]\tvocab",
      "habit\tthói quen<br>tập quán\tvocab",
      "",
    ].join("\n");

    const imported = importWordFile(anki);

    expect(imported.rows).toBe(2);
    expect(imported.text).toBe(
      "leisure\tthời gian rảnh\nhabit\tthói quen tập quán",
    );
  });

  it("reads comma CSV with quoted fields and Windows line endings", () => {
    const csv =
      '﻿journey,"chuyến đi, hành trình"\r\n"say ""hi""",chào\r\ncommute,\r\n';

    const imported = importWordFile(csv);

    expect(imported.text).toBe(
      'journey\tchuyến đi, hành trình\nsay "hi"\tchào\ncommute',
    );
  });

  it("prefers semicolon over comma when the first row uses it", () => {
    expect(importWordFile("apple;quả táo, trái táo\npear;quả lê").text).toBe(
      "apple\tquả táo, trái táo\npear\tquả lê",
    );
  });

  it("keeps a quoted newline inside one line of the box", () => {
    expect(importWordFile('word\t"line one\nline two"').text).toBe(
      "word\tline one line two",
    );
  });

  it("never turns markup into anything but text", () => {
    const imported = importWordFile(
      'xss\t<img src=x onerror="alert(1)"><script>alert(2)</script>ok',
    );
    expect(imported.text).toBe("xss\talert(2)ok");
  });

  it("produces lines the paste parser counts as words", () => {
    const imported = importWordFile("a,1\nb,2\nb,3\nc");
    expect(countWords(imported.text)).toBe(3);
  });
});

describe("splitAtLimit", () => {
  it("keeps everything when the list fits", () => {
    expect(splitAtLimit("one\ntwo", 5)).toEqual({ head: "one\ntwo", rest: "" });
  });

  it("splits before the first word past the limit, counting duplicates once", () => {
    // "42" is a page number: the parser skips a line with no letters.
    const text = "alpha\nbeta\nalpha\n\n42\ngamma\ndelta";
    const { head, rest } = splitAtLimit(text, 2);

    expect(countWords(head)).toBe(2);
    expect(head).toBe("alpha\nbeta\nalpha\n\n42");
    expect(rest).toBe("gamma\ndelta");
  });
});
