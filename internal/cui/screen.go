// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cui

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"sync"

	termbox "github.com/nsf/termbox-go"
)

var initialized = false

const padding = 2

// Point is a 2D coordinate in console
//   X is the column
//   Y is the row
type Point struct{ X, Y int }

// Rect is a 2D rectangle in console, excluding Max edge
type Rect struct{ Min, Max Point }

// Screen is a writable area on screen
type Screen struct {
	rendering sync.Mutex

	blitting sync.Mutex
	closed   bool
	flushed  frame
	pending  frame
}

type frame struct {
	size    Point
	content []byte
}

// NewScreen returns a new screen, only one screen can be use at a time.
func NewScreen() (*Screen, error) {
	if initialized {
		return nil, errors.New("only one screen allowed at a time")
	}
	initialized = true
	if err := termbox.Init(); err != nil {
		initialized = false
		return nil, err
	}

	termbox.SetInputMode(termbox.InputEsc)
	termbox.HideCursor()
	screen := &Screen{}
	screen.flushed.size.X, screen.flushed.size.Y = termbox.Size()
	screen.pending.size = screen.flushed.size
	return screen, nil
}

func (screen *Screen) markClosed() {
	screen.blitting.Lock()
	screen.closed = true
	screen.blitting.Unlock()
}

func (screen *Screen) isClosed() bool {
	screen.blitting.Lock()
	defer screen.blitting.Unlock()
	return screen.closed
}

// Close closes the screen.
func (screen *Screen) Close() error {
	screen.markClosed()

	// shutdown termbox
	termbox.Close()
	initialized = false
	return nil
}

// Run runs the event loop
func (screen *Screen) Run() error {
	defer screen.markClosed()

	for !screen.isClosed() {
		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventInterrupt:
			// either screen refresh or close
		case termbox.EventKey:
			switch ev.Key {
			case termbox.KeyCtrlC, termbox.KeyEsc:
				return nil
			default:
				// ignore key presses
			}
		case termbox.EventError:
			return ev.Err
		case termbox.EventResize:
			screen.blitting.Lock()
			screen.flushed.size.X, screen.flushed.size.Y = ev.Width, ev.Height
			err := screen.blit(&screen.flushed)
			screen.blitting.Unlock()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Size returns the current size of the screen.
func (screen *Screen) Size() (width, height int) {
	width, height = screen.pending.size.X-2*padding, screen.pending.size.Y-2*padding
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	return width, height
}

// Lock screen for exclusive rendering
func (screen *Screen) Lock() { screen.rendering.Lock() }

// Unlock screen
func (screen *Screen) Unlock() { screen.rendering.Unlock() }

// Write writes to the screen.
func (screen *Screen) Write(data []byte) (int, error) {
	screen.pending.content = append(screen.pending.content, data...)
	return len(data), nil
}

// Flush flushes pending content to the console and clears for new frame.
func (screen *Screen) Flush() error {
	screen.blitting.Lock()
	var err error
	if !screen.closed {
		err = screen.blit(&screen.pending)
	} else {
		err = context.Canceled
	}
	screen.pending.content = nil
	screen.pending.size = screen.flushed.size
	screen.blitting.Unlock()

	return err
}

// blit writes content to the console
func (screen *Screen) blit(frame *frame) error {
	screen.flushed.content = frame.content
	size := screen.flushed.size

	if err := termbox.Clear(termbox.ColorDefault, termbox.ColorDefault); err != nil {
		return err
	}

	drawRect(Rect{
		Min: Point{0, 0},
		Max: size,
	}, lightStyle)

	scanner := bufio.NewScanner(bytes.NewReader(frame.content))
	y := padding
	for scanner.Scan() && y <= size.Y-2*padding {
		x := padding
		for _, r := range scanner.Text() {
			if x > size.X-2*padding {
				break
			}
			termbox.SetCell(x, y, r, termbox.ColorDefault, termbox.ColorDefault)
			x++
		}
		y++
	}

	return termbox.Flush()
}

type rectStyle [3][3]rune

var lightStyle = rectStyle{
	{'┌', '─', '┐'},
	{'│', ' ', '│'},
	{'└', '─', '┘'},
}

// drawRect draws a rectangle using termbox
func drawRect(r Rect, style rectStyle) {
	attr := termbox.ColorDefault

	termbox.SetCell(r.Min.X, r.Min.Y, style[0][0], attr, attr)
	termbox.SetCell(r.Max.X-1, r.Min.Y, style[0][2], attr, attr)
	termbox.SetCell(r.Max.X-1, r.Max.Y-1, style[2][2], attr, attr)
	termbox.SetCell(r.Min.X, r.Max.Y-1, style[2][0], attr, attr)

	for x := r.Min.X + 1; x < r.Max.X-1; x++ {
		termbox.SetCell(x, r.Min.Y, style[0][1], attr, attr)
		termbox.SetCell(x, r.Max.Y-1, style[2][1], attr, attr)
	}

	for y := r.Min.Y + 1; y < r.Max.Y-1; y++ {
		termbox.SetCell(r.Min.X, y, style[1][0], attr, attr)
		termbox.SetCell(r.Max.X-1, y, style[1][2], attr, attr)
	}
}
