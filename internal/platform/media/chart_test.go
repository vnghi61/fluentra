package media

import (
	"strings"
	"testing"
)

func TestRenderChartSVG_DrawsEachType(t *testing.T) {
	specs := []ChartSpec{
		{ChartType: "bar", Title: "Sales", Series: []ChartSeries{{"2019", 10}, {"2020", 20}}},
		{ChartType: "line", Title: "Rainfall", Series: []ChartSeries{{"Jan", 5}, {"Feb", 8}}},
		{ChartType: "pie", Title: "Share", Series: []ChartSeries{{"A", 30}, {"B", 70}}},
	}
	for _, spec := range specs {
		t.Run(spec.ChartType, func(t *testing.T) {
			svg, err := RenderChartSVG(spec)
			if err != nil {
				t.Fatalf("a valid %s chart must render: %v", spec.ChartType, err)
			}
			out := string(svg)
			if !strings.HasPrefix(out, "<svg") {
				t.Errorf("output does not start with <svg: %s", out)
			}
			if !strings.Contains(out, spec.Title) {
				t.Errorf("output is missing the title %q", spec.Title)
			}
			if !strings.Contains(out, spec.Series[0].Label) {
				t.Errorf("output is missing the label %q", spec.Series[0].Label)
			}
		})
	}
}

func TestRenderChartSVG_RefusesDataThatDoesNotAddUp(t *testing.T) {
	tests := map[string]ChartSpec{ //nolint:gosec // fixture strings, not credentials
		"unknown type": {ChartType: "donut", Series: []ChartSeries{{"A", 1}, {"B", 2}}},
		"one point":    {ChartType: "bar", Series: []ChartSeries{{"A", 1}}},
		"negative":     {ChartType: "line", Series: []ChartSeries{{"A", -1}, {"B", 2}}},
		"empty pie":    {ChartType: "pie", Series: []ChartSeries{{"A", 0}, {"B", 0}}},
	}
	for name, spec := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := RenderChartSVG(spec); err == nil {
				t.Fatal("broken chart data must be refused")
			}
		})
	}
}
