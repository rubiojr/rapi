package main

import (
	"fmt"

	"github.com/muesli/reflow/padding"
	"github.com/muesli/termenv"
)

const (
	headerColor = "#ffb236"
	colPadding  = 20
)

func colorize(str, color string) string {
	out := termenv.String(str)
	p := termenv.ColorProfile()
	return out.Foreground(p.Color(color)).String()
}

func printRow(header, value, color string) {
	fmt.Printf("%s %s\n", padding.String(colorize(header+":", color), colPadding), value)
}
