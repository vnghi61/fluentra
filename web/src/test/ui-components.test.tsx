import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { OtpInput } from "@/components/ui/otp-input";

describe("UI Components Baseline", () => {
  describe("Button", () => {
    it("renders children and handles clicks", async () => {
      const user = userEvent.setup();
      const handleClick = vi.fn();
      render(<Button onClick={handleClick}>Click me</Button>);

      const btn = screen.getByRole("button", { name: /click me/i });
      expect(btn).toBeInTheDocument();
      await user.click(btn);
      expect(handleClick).toHaveBeenCalledTimes(1);
    });

    it("disables button and sets aria-busy when isLoading", () => {
      render(<Button isLoading>Submitting</Button>);
      const btn = screen.getByRole("button", { name: /submitting/i });
      expect(btn).toBeDisabled();
      expect(btn).toHaveAttribute("aria-busy", "true");
    });
  });

  describe("Input", () => {
    it("renders with accessible attributes and handles value changes", async () => {
      const user = userEvent.setup();
      const handleChange = vi.fn();
      render(
        <Input
          placeholder="Enter email"
          aria-label="Email"
          onChange={handleChange}
        />,
      );

      const input = screen.getByLabelText("Email");
      expect(input).toBeInTheDocument();
      await user.type(input, "test@example.com");
      expect(handleChange).toHaveBeenCalled();
    });

    it("sets aria-invalid when error is true", () => {
      render(<Input aria-label="Password" error />);
      const input = screen.getByLabelText("Password");
      expect(input).toHaveAttribute("aria-invalid", "true");
    });
  });

  describe("Checkbox", () => {
    function ControlledCheckbox() {
      const [checked, setChecked] = useState(false);
      return (
        <Checkbox
          checked={checked}
          onCheckedChange={setChecked}
          label="Remember device"
        />
      );
    }

    it("toggles checked state on click", async () => {
      const user = userEvent.setup();
      render(<ControlledCheckbox />);

      const checkbox = screen.getByRole("checkbox");
      expect(checkbox).not.toBeChecked();

      await user.click(checkbox);
      expect(checkbox).toBeChecked();

      await user.click(checkbox);
      expect(checkbox).not.toBeChecked();
    });
  });

  describe("OtpInput", () => {
    function ControlledOtp({
      onComplete,
    }: {
      onComplete?: (code: string) => void;
    }) {
      const [value, setValue] = useState("");
      return (
        <OtpInput value={value} onChange={setValue} onComplete={onComplete} />
      );
    }

    it("renders 6 segmented inputs with numeric inputmode and one-time-code autocomplete", () => {
      render(<ControlledOtp />);
      const inputs = screen.getAllByRole("textbox");
      expect(inputs).toHaveLength(6);

      expect(inputs[0]).toHaveAttribute("inputmode", "numeric");
      expect(inputs[0]).toHaveAttribute("autocomplete", "one-time-code");
      expect(inputs[1]).toHaveAttribute("autocomplete", "off");
    });

    it("fills all 6 digits when pasted and fires onComplete", async () => {
      const user = userEvent.setup();
      const handleComplete = vi.fn();
      render(<ControlledOtp onComplete={handleComplete} />);

      const inputs = screen.getAllByRole("textbox");
      await user.click(inputs[0]!);
      await user.paste("654321");

      expect(inputs[0]).toHaveValue("6");
      expect(inputs[1]).toHaveValue("5");
      expect(inputs[2]).toHaveValue("4");
      expect(inputs[3]).toHaveValue("3");
      expect(inputs[4]).toHaveValue("2");
      expect(inputs[5]).toHaveValue("1");

      expect(handleComplete).toHaveBeenCalledWith("654321");
    });
  });
});
