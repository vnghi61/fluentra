# Free Spaced Repetition Scheduler (FSRS) Algorithm

> Technical reference for the FSRS algorithm implementation in Fluentra.
> Single source of truth for scheduling formulas, parameters, and memory mechanics.
> Referenced by [`ADR-0016`](../adr/ADR-0016-srs-fsrs.md) and [`internal/modules/srs/AGENT.md`](../../internal/modules/srs/AGENT.md).

---

## 1. Overview & Theoretical Foundation

FSRS (Free Spaced Repetition Scheduler) is a modern spaced repetition algorithm based on the **Three-Component Model of Memory**:

1. **Stability ($S$)**: Time (in days) required for the retrievability (probability of recall) to drop from $100\%$ to $90\%$.
2. **Difficulty ($D$)**: Inherent complexity of the learnable item on a scale from $1.0$ (easiest) to $10.0$ (hardest).
3. **Retrievability ($R$)**: The probability that the learner successfully recalls the item at elapsed time $t$.

Compared to legacy SM-2 (which uses a single scalar ease factor), FSRS explicitly models memory decay and item difficulty, reducing redundant reviews by 20–30% for equivalent retention.

---

## 2. Mathematical Model & Formulas

### 2.1 Forgetting Curve & Retrievability

FSRS models memory decay using a power forgetting curve:

$$R(t, S) = \left(1 + \text{FACTOR} \cdot \frac{t}{S}\right)^{-0.5}$$

where:

- $t$: Elapsed time in days since the last review.
- $S$: Memory stability in days.
- $\text{FACTOR} = \frac{19}{81} \approx 0.2345679$, derived from $(1 + \text{FACTOR})^{-0.5} = 0.90$, ensuring $R(S, S) = 0.90$.

---

### 2.2 Initial State ($G \in \{1, 2, 3, 4\}$)

Upon first encounter with an item ($G = 1: \text{Again}, 2: \text{Hard}, 3: \text{Good}, 4: \text{Easy}$):

- **Initial Stability**:
  $$S_0(G) = w_{G-1}$$

- **Initial Difficulty**:
  $$D_0(G) = \text{clamp}\left(w_4 - e^{w_5 \cdot (G - 1)} + 1, 1.0, 10.0\right)$$

---

### 2.3 Difficulty Updating

After subsequent reviews:

$$\Delta D = -w_6 \cdot (G - 3)$$
$$D' = D + \Delta D \cdot \left(\frac{10 - D}{9}\right)$$
$$D_{\text{next}} = \text{clamp}\left(w_7 \cdot D_0(3) + (1 - w_7) \cdot D', 1.0, 10.0\right)$$

- Mean reversion ($w_7$) gently pulls extreme difficulty ratings toward the global mean over time.

---

### 2.4 Stability Updating

#### A. On Recall Success ($G \in \{2, 3, 4\}$)

$$S'_r(D, S, R, G) = S \cdot \left(1 + e^{w_8} \cdot (11 - D) \cdot S^{-w_9} \cdot \left(e^{w_{10} \cdot (1 - R)} - 1\right) \cdot h(G)\right)$$

where:

- $h(\text{Hard}) = w_{15}$ (Hard penalty, $< 1.0$)
- $h(\text{Good}) = 1.0$
- $h(\text{Easy}) = w_{16}$ (Easy bonus, $> 1.0$)

#### B. On Recall Lapse ($G = 1: \text{Again}$)

When a learner fails to recall a mature card:

$$S'_f(D, S, R) = w_{11} \cdot D^{-w_{12}} \cdot \left((S + 1)^{w_{13}} - 1\right) \cdot e^{w_{14} \cdot (1 - R)}$$

- Stability is reduced according to the lapse formula, subject to $0.1 \le S'_f \le S$. Prior memory is not reset to zero.

---

### 2.5 Scheduling Interval Calculation

Given target request retention $r$ (default $0.90$):

$$I(S, r) = \text{round}\left(\frac{S}{\text{FACTOR}} \cdot \left(r^{-2} - 1\right)\right)$$

When $r = 0.90$, $I(S, 0.90) = S$.

Interval is clamped: $1 \le I \le \text{MaxInterval}$.

---

## 3. Standard Published Parameters (FSRS v4.5)

| Weight | Default Value | Purpose |
|---|---|---|
| $w_0$ | `0.4072` | Initial stability for $\text{Again}$ |
| $w_1$ | `1.1829` | Initial stability for $\text{Hard}$ |
| $w_2$ | `3.1262` | Initial stability for $\text{Good}$ |
| $w_3$ | `15.4722` | Initial stability for $\text{Easy}$ |
| $w_4$ | `7.2102` | Initial difficulty base |
| $w_5$ | `0.5316` | Initial difficulty grade step factor |
| $w_6$ | `1.0651` | Difficulty update scaling factor |
| $w_7$ | `0.0234` | Difficulty mean reversion weight |
| $w_8$ | `1.6160` | Stability increase base factor |
| $w_9$ | `0.1544` | Stability increase power factor on $S$ |
| $w_{10}$ | `1.0824` | Retrievability factor on stability increase |
| $w_{11}$ | `1.9813` | Lapse stability base factor |
| $w_{12}$ | `0.0953` | Difficulty power on lapse stability |
| $w_{13}$ | `0.2975` | Stability power on lapse stability |
| $w_{14}$ | `2.2042` | Retrievability factor on lapse stability |
| $w_{15}$ | `0.2407` | $\text{Hard}$ rating stability penalty multiplier |
| $w_{16}$ | `2.9466` | $\text{Easy}$ rating stability bonus multiplier |
| $w_{17}$ | `0.5034` | Short-term stability factor 1 |
| $w_{18}$ | `0.6567` | Short-term stability factor 2 |

> [!WARNING]
> These parameters form a unified system optimized against review benchmarks. They are not independent knobs; altering individual weights without full dataset simulation will degrade scheduling accuracy.

---

## 4. Implementation Guidelines in Fluentra

1. **Pure Domain Functions**: All scheduling operations live in `internal/modules/srs/domain/fsrs.go` with zero database or clock dependencies.
2. **Deterministic Inputs**: Current time `now` is explicitly passed as an argument.
3. **Property Invariants**:
   - $I(\text{Easy}) \ge I(\text{Good}) \ge I(\text{Hard}) \ge I(\text{Again})$
   - Stability is strictly monotonic in interval length.
   - A lapse reduces stability without zeroing it.
   - All intervals for passing grades are positive integer days ($\ge 1$).
