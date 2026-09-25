package media

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"math"
	"strings"
)

// ChartSpec is the data an IELTS Writing Task 1 visual carries: the generator
// returns it, our code draws it, and the body keeps it so the grader can read
// the numbers a learner describes (WO 22 D22-22).
type ChartSpec struct {
	ChartType string        `json:"chart_type"`
	Title     string        `json:"title"`
	Series    []ChartSeries `json:"series"`
}

// ChartSeries is one bar, one point or one slice.
type ChartSeries struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

const (
	chartBar  = "bar"
	chartLine = "line"
	chartPie  = "pie"

	chartWidth  = 640
	chartHeight = 360
	chartMargin = 48
)

// RenderChartSVG draws the chart as an SVG. It refuses data that does not add
// up — an unknown chart type, fewer than two points, a negative value, or a pie
// whose values are all zero — so a Task 1 whose visual is broken fails before a
// learner sees it.
func RenderChartSVG(spec ChartSpec) ([]byte, error) {
	if err := spec.validate(); err != nil {
		return nil, err
	}
	switch spec.ChartType {
	case chartBar:
		return renderBarChart(spec), nil
	case chartLine:
		return renderLineChart(spec), nil
	case chartPie:
		return renderPieChart(spec), nil
	default:
		return nil, fmt.Errorf("unknown chart type %q", spec.ChartType)
	}
}

// ChartDataURI draws a chart into an SVG data URI. The drawing is a few
// kilobytes and travels inside the item's body, so a learner sees it wherever
// the body goes with no stored object to sign or expire; the page's policy
// already allows data: images.
type ChartDataURI struct{}

// RenderChart decodes the model's chart data, draws it and returns the image.
func (ChartDataURI) RenderChart(raw json.RawMessage) (string, error) {
	var spec ChartSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return "", fmt.Errorf("decode chart: %w", err)
	}
	svg, err := RenderChartSVG(spec)
	if err != nil {
		return "", err
	}
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(svg), nil
}

func (s ChartSpec) validate() error {
	switch s.ChartType {
	case chartBar, chartLine, chartPie:
	default:
		return fmt.Errorf("unknown chart type %q", s.ChartType)
	}
	if len(s.Series) < 2 {
		return errors.New("a chart needs at least two points")
	}
	total := 0.0
	for _, point := range s.Series {
		if math.IsNaN(point.Value) || math.IsInf(point.Value, 0) || point.Value < 0 {
			return fmt.Errorf("value for %q must be a non-negative number", point.Label)
		}
		total += point.Value
	}
	if s.ChartType == chartPie && total <= 0 {
		return errors.New("a pie chart's values must not all be zero")
	}
	return nil
}

func (s ChartSpec) maxValue() float64 {
	maxValue := 0.0
	for _, point := range s.Series {
		if point.Value > maxValue {
			maxValue = point.Value
		}
	}
	if maxValue == 0 {
		return 1
	}
	return maxValue
}

func svgHeader(spec ChartSpec, body strings.Builder) []byte {
	var out strings.Builder
	out.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 `)
	fmt.Fprintf(&out, "%d %d", chartWidth, chartHeight)
	out.WriteString(`" role="img" font-family="system-ui, sans-serif">`)
	if spec.Title != "" {
		out.WriteString(`<text x="`)
		fmt.Fprintf(&out, "%d", chartWidth/2)
		out.WriteString(`" y="24" text-anchor="middle" font-size="16" font-weight="700">`)
		out.WriteString(html.EscapeString(spec.Title))
		out.WriteString(`</text>`)
	}
	out.WriteString(body.String())
	out.WriteString(`</svg>`)
	return []byte(out.String())
}

func plotArea() (x, y, w, h float64) {
	return chartMargin, 40, chartWidth - 2*chartMargin, chartHeight - 2*chartMargin
}

func renderBarChart(spec ChartSpec) []byte {
	x, y, w, h := plotArea()
	barWidth := w / float64(len(spec.Series))
	maxValue := spec.maxValue()

	var body strings.Builder
	body.WriteString(`<line x1="`)
	fmt.Fprintf(&body, "%.0f\" y1=\"%.0f\" x2=\"%.0f\" y2=\"%.0f\" stroke=\"#94a3b8\"/>", x, y+h, x+w, y+h)
	for i, point := range spec.Series {
		barHeight := (point.Value / maxValue) * h
		bx := x + float64(i)*barWidth + barWidth*0.15
		by := y + h - barHeight
		fmt.Fprintf(&body, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="#2563eb"/>`,
			bx, by, barWidth*0.7, barHeight)
		fmt.Fprintf(&body, `<text x="%.1f" y="%.0f" text-anchor="middle" font-size="11" fill="#475569">%s</text>`,
			x+float64(i)*barWidth+barWidth/2, y+h+16, html.EscapeString(point.Label))
	}
	return svgHeader(spec, body)
}

func renderLineChart(spec ChartSpec) []byte {
	x, y, w, h := plotArea()
	maxValue := spec.maxValue()
	step := w / float64(len(spec.Series)-1)

	var body strings.Builder
	body.WriteString(`<polyline fill="none" stroke="#2563eb" stroke-width="2" points="`)
	for i, point := range spec.Series {
		px := x + float64(i)*step
		py := y + h - (point.Value/maxValue)*h
		fmt.Fprintf(&body, "%.1f,%.1f ", px, py)
	}
	body.WriteString(`"/>`)
	for i, point := range spec.Series {
		px := x + float64(i)*step
		py := y + h - (point.Value/maxValue)*h
		fmt.Fprintf(&body, `<circle cx="%.1f" cy="%.1f" r="3" fill="#2563eb"/>`, px, py)
		fmt.Fprintf(&body, `<text x="%.1f" y="%.0f" text-anchor="middle" font-size="11" fill="#475569">%s</text>`,
			px, y+h+16, html.EscapeString(point.Label))
	}
	return svgHeader(spec, body)
}

func renderPieChart(spec ChartSpec) []byte {
	cx, cy := float64(chartWidth)/2, float64(chartHeight)/2+8
	radius := math.Min(float64(chartWidth), float64(chartHeight))/2 - chartMargin
	total := 0.0
	for _, point := range spec.Series {
		total += point.Value
	}

	var body strings.Builder
	start := -math.Pi / 2
	for i, point := range spec.Series {
		angle := (point.Value / total) * 2 * math.Pi
		end := start + angle
		largeArc := 0
		if angle > math.Pi {
			largeArc = 1
		}
		x1 := cx + radius*math.Cos(start)
		y1 := cy + radius*math.Sin(start)
		x2 := cx + radius*math.Cos(end)
		y2 := cy + radius*math.Sin(end)
		fmt.Fprintf(&body,
			`<path d="M %.1f %.1f L %.1f %.1f A %.1f %.1f 0 %d 1 %.1f %.1f Z" fill="%s"/>`,
			cx, cy, x1, y1, radius, radius, largeArc, x2, y2, pieColour(i))
		start = end
	}
	return svgHeader(spec, body)
}

func pieColour(i int) string {
	colours := []string{"#2563eb", "#16a34a", "#f59e0b", "#dc2626", "#7c3aed", "#0891b2"}
	return colours[i%len(colours)]
}
