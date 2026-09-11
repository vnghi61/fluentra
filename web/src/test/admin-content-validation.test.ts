import { describe, expect, it } from "vitest";

import { validateContentBody } from "@/features/admin/components/AdminContentDetailModal";

describe("AdminContentDetailModal validateContentBody", () => {
  describe("reading_comprehension", () => {
    it("rejects when passage is missing", () => {
      const body = JSON.stringify({
        questions: [{ id: "q1", answer: "A" }],
      });
      const result = validateContentBody("reading_comprehension", body);
      expect(result.valid).toBe(false);
      expect(result.error).toContain("Reading comprehension exercises require a non-empty 'passage'");
    });

    it("rejects when questions is empty array", () => {
      const body = JSON.stringify({
        passage: "Here is a reading passage.",
        questions: [],
      });
      const result = validateContentBody("reading_comprehension", body);
      expect(result.valid).toBe(false);
      expect(result.error).toContain("must include at least one question");
    });

    it("rejects when a question in questions array has no answer", () => {
      const body = JSON.stringify({
        passage: "Here is a reading passage.",
        questions: [
          { id: "q1", prompt: "Question 1", answer: "opt_a" },
          { id: "q2", prompt: "Question 2 without answer" },
        ],
      });
      const result = validateContentBody("reading_comprehension", body);
      expect(result.valid).toBe(false);
      expect(result.error).toContain("must specify an answer");
    });

    it("accepts valid multi-question comprehension body", () => {
      const body = JSON.stringify({
        passage: "Here is a valid reading passage about science and discovery.",
        questions: [
          { id: "q1", prompt: "Q1", answer: "opt_a" },
          { id: "q2", prompt: "Q2", correct_answer: "true" },
          { id: "q3", prompt: "Q3", correct_option_id: "opt_c" },
        ],
      });
      const result = validateContentBody("reading_comprehension", body);
      expect(result.valid).toBe(true);
    });

    it("accepts valid legacy single-question comprehension body", () => {
      const body = JSON.stringify({
        passage: "Here is a legacy passage.",
        prompt: "Why?",
        correct_answer: "opt_1",
      });
      const result = validateContentBody("reading_comprehension", body);
      expect(result.valid).toBe(true);
    });

    it("rejects legacy single-question comprehension without answer", () => {
      const body = JSON.stringify({
        passage: "Here is a legacy passage.",
        prompt: "Why?",
      });
      const result = validateContentBody("reading_comprehension", body);
      expect(result.valid).toBe(false);
      expect(result.error).toContain("Reading comprehension requires 'correct_answer'");
    });
  });

  describe("writing_prompt", () => {
    it("rejects when prompt is missing", () => {
      const body = JSON.stringify({
        min_words: 60,
      });
      const result = validateContentBody("writing_prompt", body);
      expect(result.valid).toBe(false);
      expect(result.error).toContain("Writing prompt exercises require a non-empty 'prompt'");
    });

    it("rejects when min_words is missing or <= 0", () => {
      const body = JSON.stringify({
        prompt: "Write an email to a friend.",
        min_words: 0,
      });
      const result = validateContentBody("writing_prompt", body);
      expect(result.valid).toBe(false);
      expect(result.error).toContain("Writing prompt exercises require 'min_words' to be greater than zero");
    });

    it("accepts valid writing prompt body", () => {
      const body = JSON.stringify({
        prompt: "Write an opinion essay on remote working.",
        min_words: 120,
        sample_answer: "In my opinion, remote working offers significant flexibility...",
      });
      const result = validateContentBody("writing_prompt", body);
      expect(result.valid).toBe(true);
    });
  });
});
