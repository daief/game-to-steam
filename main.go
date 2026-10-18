// game-to-steam is a native desktop application for managing Steam shortcuts.
package main

import (
	"log"

	"game-to-steam/internal/ui"
)

func main() {
	if err := ui.Run(); err != nil {
		log.Fatal(err)
	}
}
