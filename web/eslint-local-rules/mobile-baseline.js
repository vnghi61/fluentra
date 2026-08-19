// Local ESLint rules enforcing ADR-0024's two hard limits (web/AGENT.md §6b):
//
//   R1 — touch targets must be at least 44×44 CSS px.
//   R2 — input font-size must be at least 16 px (below this iOS Safari zooms
//        the viewport on focus, and the learner has to pinch back out mid-form).
//
// These are lint rules rather than guidelines on purpose: a guideline in a
// document drifts, and this project has found that repeatedly. A rule that runs
// on `pnpm run lint` fails the PR the moment a target shrinks under the limit.
//
// The rules inspect Tailwind utility classes in `className` string literals.
// Dynamically computed class names (clsx/`cn` calls, template literals) are
// skipped: a class the linter cannot read is a class the linter cannot vouch
// for, but flagging every conditional class would flood the report with noise.

/** Tailwind default spacing scale, token -> px (1 = 0.25rem = 4px). */
const SPACING_PX = {
  '0': 0, px: 1, '0.5': 2, '1': 4, '1.5': 6, '2': 8, '2.5': 10, '3': 12,
  '3.5': 14, '4': 16, '5': 20, '6': 24, '7': 28, '8': 32, '9': 36, '10': 40,
  '11': 44, '12': 48, '14': 56, '16': 64, '20': 80, '24': 96, '28': 112,
  '32': 128, '36': 144, '40': 160, '44': 176, '48': 192, '52': 208, '56': 224,
  '60': 240, '64': 256, '72': 288, '80': 320, '96': 384,
};

/** Tailwind default font-size scale, token -> px. */
const TEXT_PX = {
  xs: 12, sm: 14, base: 16, lg: 18, xl: 20, '2xl': 24, '3xl': 30, '4xl': 36,
  '5xl': 48, '6xl': 60, '7xl': 72, '8xl': 96, '9xl': 128,
};

const MIN_TOUCH_PX = 44;
const MIN_INPUT_FONT_PX = 16;

/** Returns the raw value inside square brackets, or null. */
function arbitraryPx(token) {
  const match = /^\[(.+)\]$/.exec(token);
  if (!match) return null;
  const value = match[1];
  const px = /^(-?\d+(?:\.\d+)?)px$/.exec(value);
  if (px) return Number(px[1]);
  const rem = /^(-?\d+(?:\.\d+)?)rem$/.exec(value);
  if (rem) return Number(rem[1]) * 16;
  return null;
}

function spacingPx(token) {
  if (SPACING_PX[token] !== undefined) return SPACING_PX[token];
  return arbitraryPx(token);
}

function textPx(token) {
  if (TEXT_PX[token] !== undefined) return TEXT_PX[token];
  return arbitraryPx(token);
}

/** The declared width/height of a target in a given axis, in px, or null. */
function axisPx(tokens, axis) {
  // min-* wins: a target with `min-h-[44px]` satisfies R1 however small the
  // bare `h-*` class is, because the computed height is bounded below at 44.
  for (const token of tokens) {
    if (token.startsWith(`min-${axis}-`)) {
      const px = spacingPx(token.slice(`min-${axis}-`.length));
      if (px !== null) return px;
    }
  }
  for (const token of tokens) {
    if (token.startsWith(`${axis}-`)) {
      const px = spacingPx(token.slice(`${axis}-`.length));
      if (px !== null) return px;
    }
  }
  return null;
}

function textSizePx(tokens) {
  for (const token of tokens) {
    if (token.startsWith('text-')) {
      const px = textPx(token.slice('text-'.length));
      if (px !== null) return px;
    }
  }
  return null;
}

function classNameTokens(node) {
  for (const attr of node.attributes) {
    if (
      attr.type === 'JSXAttribute' &&
      attr.name &&
      attr.name.name === 'className' &&
      attr.value
    ) {
      const value = attr.value;
      if (value.type === 'Literal' && typeof value.value === 'string') {
        return value.value.split(/\s+/).filter(Boolean);
      }
      // The codebase composes class names with `cn(...)`/`clsx(...)` more often
      // than it writes a literal. Walk the call's string literals — including
      // array elements and both branches of a conditional — so a class hidden
      // inside the helper is still enforced. A class behind a variable we cannot
      // read is skipped, not trusted.
      if (value.type === 'JSXExpressionContainer') {
        const strings = [];
        collectStrings(value.expression, strings);
        if (strings.length > 0) return strings.join(' ').split(/\s+/).filter(Boolean);
      }
      return null;
    }
  }
  return null;
}

function collectStrings(expr, out) {
  if (!expr) return;
  switch (expr.type) {
    case 'Literal':
      if (typeof expr.value === 'string') out.push(expr.value);
      return;
    case 'TemplateLiteral':
      if (expr.expressions.length === 0) out.push(expr.quasis[0].value.cooked ?? '');
      return;
    case 'ArrayExpression':
      for (const element of expr.elements) collectStrings(element, out);
      return;
    case 'ConditionalExpression':
      collectStrings(expr.consequent, out);
      collectStrings(expr.alternate, out);
      return;
    case 'LogicalExpression':
      collectStrings(expr.left, out);
      collectStrings(expr.right, out);
      return;
    case 'CallExpression':
      for (const arg of expr.arguments) collectStrings(arg, out);
      return;
    default:
      return;
  }
}

function isInteractive(node) {
  if (node.name && node.name.type === 'JSXIdentifier') {
    const tag = node.name.name;
    if (['button', 'a', 'input', 'select', 'textarea'].includes(tag)) return true;
  }
  for (const attr of node.attributes) {
    if (attr.type !== 'JSXAttribute' || !attr.name || attr.name.type !== 'JSXIdentifier') {
      continue;
    }
    const name = attr.name.name;
    if (name === 'onClick' || name === 'onPress') return true;
    if (name === 'role' && attr.value && attr.value.type === 'Literal' && attr.value.value === 'button') {
      return true;
    }
  }
  return false;
}

function isTextInput(node) {
  if (!node.name || node.name.type !== 'JSXIdentifier') return false;
  return ['input', 'select', 'textarea'].includes(node.name.name);
}

export default {
  rules: {
    'touch-target': {
      meta: { type: 'problem', docs: { description: 'Enforce 44×44 px minimum touch targets (R1)' } },
      create(context) {
        return {
          JSXOpeningElement(node) {
            if (!isInteractive(node)) return;
            const tokens = classNameTokens(node);
            if (!tokens) return;

            const height = axisPx(tokens, 'h');
            const width = axisPx(tokens, 'w');
            const undersized = [];
            if (height !== null && height < MIN_TOUCH_PX) undersized.push(`height ${height}px`);
            if (width !== null && width < MIN_TOUCH_PX) undersized.push(`width ${width}px`);
            if (undersized.length === 0) return;

            context.report({
              node,
              message: `Touch target is smaller than ${MIN_TOUCH_PX}px (${undersized.join(', ')}). Interactive elements must be at least 44×44 CSS px (web/AGENT.md R1).`,
            });
          },
        };
      },
    },
    'input-font-size': {
      meta: { type: 'problem', docs: { description: 'Enforce 16 px minimum input font-size (R2)' } },
      create(context) {
        return {
          JSXOpeningElement(node) {
            if (!isTextInput(node)) return;
            const tokens = classNameTokens(node);
            if (!tokens) return;

            const size = textSizePx(tokens);
            if (size === null || size >= MIN_INPUT_FONT_PX) return;

            context.report({
              node,
              message: `Input font-size is ${size}px, below the ${MIN_INPUT_FONT_PX}px minimum. iOS Safari zooms the viewport on focus below this (web/AGENT.md R2).`,
            });
          },
        };
      },
    },
  },
};
