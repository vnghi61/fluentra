import * as React from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, beforeAll, afterAll } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { useForm } from "react-hook-form";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import i18n from "i18next";
import { initReactI18next, I18nextProvider } from "react-i18next";

import en from "@/i18n/en.json";
import vi from "@/i18n/vi.json";
import { LoginForm } from "@/features/auth/components/LoginForm";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";

function resolve(bundle: unknown, key: string): unknown {
  return key
    .split(".")
    .reduce<unknown>(
      (node, part) =>
        typeof node === "object" && node !== null
          ? (node as Record<string, unknown>)[part]
          : undefined,
      bundle,
    );
}

async function renderWithRouter(ui: React.ReactElement) {
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => ui,
  });
  const registerRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/register",
    component: () => <div>Register</div>,
  });
  const loginRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/login",
    component: () => <div>Login</div>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute, registerRoute, loginRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
  });
  await router.load();
  return render(
    <I18nextProvider i18n={i18n}>
      <RouterProvider router={router} />
    </I18nextProvider>,
  );
}

function TestFormMessageComponent({ errorMessage }: { errorMessage?: string }) {
  const form = useForm<{ testField: string }>({
    defaultValues: { testField: "" },
  });

  React.useEffect(() => {
    if (errorMessage) {
      form.setError("testField", { message: errorMessage });
    }
  }, [errorMessage, form]);

  return (
    <I18nextProvider i18n={i18n}>
      <Form {...form}>
        <form>
          <FormField
            control={form.control}
            name="testField"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Test Field</FormLabel>
                <FormControl>
                  <input data-testid="test-input" {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </form>
      </Form>
    </I18nextProvider>
  );
}

describe("Translatable Form Validation Messages (TASK-I18N-B)", () => {
  beforeAll(async () => {
    if (!i18n.isInitialized) {
      await i18n.use(initReactI18next).init({
        lng: "vi",
        resources: {
          en: { translation: en },
          vi: { translation: vi },
        },
        fallbackLng: "en",
        interpolation: { escapeValue: false },
      });
    } else {
      i18n.addResourceBundle("en", "translation", en, true, true);
      i18n.addResourceBundle("vi", "translation", vi, true, true);
      await i18n.changeLanguage("vi");
    }
  });

  afterAll(async () => {
    if (i18n.isInitialized) {
      await i18n.changeLanguage("en");
    }
  });

  it("every validation.* key referenced in the schemas resolves in both locales", () => {
    const schemaPaths = [
      join(process.cwd(), "src/features/auth/model/schemas.ts"),
      join(process.cwd(), "src/features/account/model/schemas.ts"),
      join(process.cwd(), "src/features/admin/model/schemas.ts"),
    ];

    const referencedKeys = new Set<string>();
    const VALIDATION_KEY_REGEX = /"validation\.([a-zA-Z0-9]+)"/g;

    for (const filePath of schemaPaths) {
      const content = readFileSync(filePath, "utf-8");
      for (const match of content.matchAll(VALIDATION_KEY_REGEX)) {
        referencedKeys.add(`validation.${match[1]}`);
      }
    }

    expect(referencedKeys.size).toBeGreaterThanOrEqual(25);

    for (const key of referencedKeys) {
      const enValue = resolve(en, key);
      const viValue = resolve(vi, key);

      expect(enValue, `Missing English translation for ${key}`).toBeDefined();
      expect(typeof enValue).toBe("string");
      expect((enValue as string).trim().length).toBeGreaterThan(0);

      expect(
        viValue,
        `Missing Vietnamese translation for ${key}`,
      ).toBeDefined();
      expect(typeof viValue).toBe("string");
      expect((viValue as string).trim().length).toBeGreaterThan(0);

      expect(
        enValue,
        `Expected distinct translations in en and vi for ${key}`,
      ).not.toBe(viValue);
    }
  });

  it("FormMessage translates validation.* keys and passes non-key strings through unchanged", () => {
    const { unmount } = render(
      <TestFormMessageComponent errorMessage="validation.emailRequired" />,
    );

    expect(screen.getByText("Email là bắt buộc")).toBeInTheDocument();
    expect(screen.queryByText("Email is required")).not.toBeInTheDocument();
    unmount();

    render(
      <TestFormMessageComponent errorMessage="Server rejected with 422: Custom error" />,
    );

    expect(
      screen.getByText("Server rejected with 422: Custom error"),
    ).toBeInTheDocument();
  });

  it("login with an empty email shows the Vietnamese message under vi", async () => {
    const user = userEvent.setup();
    await renderWithRouter(<LoginForm />);

    const submitBtn = screen.getByRole("button", {
      name: /đăng nhập|sign in/i,
    });
    await user.click(submitBtn);

    await waitFor(() => {
      expect(screen.getByText("Email là bắt buộc")).toBeInTheDocument();
    });

    expect(screen.queryByText("Email is required")).not.toBeInTheDocument();
  });
});
