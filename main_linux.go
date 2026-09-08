//go:build !windows

package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: obj2png <input.obj> <output.png>")
		os.Exit(2)
	}
	o := DefaultRenderOptions()
	o.Width = 1200
	o.Height = 600
	o.AA = 1
	if err := RenderOBJ(os.Args[1], os.Args[2], o); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
