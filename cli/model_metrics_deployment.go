package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/beamlit/toolkit/sdk"
	"github.com/spf13/cobra"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

func (r *Operations) MetricsModelDeploymentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "metrics [name]",
		Args:  cobra.MaximumNArgs(2),
		Short: "Get metrics for a model deployment",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			if len(args) == 0 {
				fmt.Println("Error: Name of the model is required")
				os.Exit(1)
			}

			res, err := client.GetModelDeploymentMetrics(ctx, args[0], environment)
			if err != nil {
				panic(err)
			}

			body, err := io.ReadAll(res.Body)
			if err != nil {
				panic(err)
			}

			var metric sdk.ModelDeploymentMetrics
			err = json.Unmarshal(body, &metric)
			if err != nil {
				panic(err)
			}
			p := plot.New()

			p.Title.Text = fmt.Sprintf("Model Deployment: %s", args[0])
			p.X.Label.Text = "Time"
			p.Y.Label.Text = "RPS"
			line, err := plotter.NewLine(getPoints(*metric.InferencePerSecondGlobal))
			if err != nil {
				panic(err)
			}
			line.Color = color.RGBA{R: 255, A: 255}
			p.Add(line)
			p.X.Tick.Marker = plot.TimeTicks{Format: "2006-01-02\n15:04"}
			// Save the plot to a PNG file

			if err := p.Save(6*vg.Inch, 4*vg.Inch, "/tmp/beamlit-metrics.png"); err != nil {
				panic(err)
			}

			// Open the PNG file with the system's default image viewer
			var openFileCmd *exec.Cmd
			switch runtime.GOOS {
			case "darwin":
				openFileCmd = exec.Command("open", "/tmp/beamlit-metrics.png")
			case "linux":
				openFileCmd = exec.Command("xdg-open", "/tmp/beamlit-metrics.png")
			case "windows":
				openFileCmd = exec.Command("cmd", "/c", "start", "/tmp/beamlit-metrics.png")
			default:
				fmt.Println("Unsupported operating system")
				return
			}

			if err := openFileCmd.Run(); err != nil {
				fmt.Printf("Error opening plot: %v\n", err)
			}
		},
	}
}

func getPoints(data sdk.ArrayMetric) plotter.XYs {
	pts := make(plotter.XYs, len(data))
	for i := range data {
		timestamp, err := time.Parse(time.RFC3339, *data[i].Timestamp)
		if err != nil {
			panic(err)
		}
		pts[i].X = float64(timestamp.Unix())
		value, err := strconv.ParseFloat(*data[i].Value, 64)
		if err != nil {
			panic(err)
		}
		pts[i].Y = value
	}
	return pts
}
