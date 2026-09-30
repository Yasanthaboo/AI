package analysis

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"regexp"
	"strconv"
)

var mediaBoxPattern = regexp.MustCompile(`/MediaBox\s*\[\s*(-?[\d.]+)\s+(-?[\d.]+)\s+(-?[\d.]+)\s+(-?[\d.]+)\s*\]`)
var rotatePattern = regexp.MustCompile(`/Rotate\s+(-?\d+)`)

// drawingAspect returns height/width of the drawing (first PDF page box), or 0 when unknown.
func drawingAspect(content []byte) float64 {
	if config, _, err := image.DecodeConfig(bytes.NewReader(content)); err == nil && config.Width > 0 {
		return float64(config.Height) / float64(config.Width)
	}
	match := mediaBoxPattern.FindSubmatch(content)
	if match == nil {
		return 0
	}
	var box [4]float64
	for index := range box {
		value, err := strconv.ParseFloat(string(match[index+1]), 64)
		if err != nil {
			return 0
		}
		box[index] = value
	}
	width, height := math.Abs(box[2]-box[0]), math.Abs(box[3]-box[1])
	if width == 0 || height == 0 {
		return 0
	}
	if rotation := rotatePattern.FindSubmatch(content); rotation != nil {
		if degrees, err := strconv.Atoi(string(rotation[1])); err == nil && (degrees/90)%2 != 0 {
			width, height = height, width
		}
	}
	return height / width
}
