# go-torblock

Tor node blocking middleware for [Negroni](https://github.com/codegangsta/negroni).

Blocks requests from knowns Tor exit nodes, periodically updates the list of known nodes in the background from [the Tor bulk exit list](https://check.torproject.org/cgi-bin/TorBulkExitList.py).

Structure borrows *liberally* from [unrolled/secure](https://github.com/unrolled/secure).

Generally speaking this is better done in a firewall, but is useful in environments where that level of control is either unavailable or undesired.

## Usage

```go
package main

import (
	"net/http"
	"github.com/duggan/go-torblock"
)

func myApp(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello world!"))
}

func main() {
	myHandler := http.HandlerFunc(myApp)

	torBlocker := torblock.New(torblock.Options{
		UpdateFrequency: 60,
		CheckURL:        "https://gist.githubusercontent.com/duggan/d60e32fb97ab9e0abdbe/raw/exit-addresses",
	})
	torBlocker.Run()

	app := torBlocker.Handler(myHandler)
	http.ListenAndServe("0.0.0.0:3000", app)
}

```

The `.Run()` directive is critical, without it, options are not loaded and the background retrieval of the exit node list is not initiated.

## Options
```go
// ...
torBlocker := torblock.New(torblock.Options{
	UpdateFrequency: 3600, // Instructs the background ticker how many seconds to wait between updates. Default 3600 (one hour).
	CheckURL: "https://check.torproject.org/exit-addresses" // The address to pull the exit node list from. Useful for proxying, caching, or debugging.
})
// ...
```

## Integration

### [Negroni](https://github.com/codegangsta/negroni)
Note this implementation has a special helper function called `HandlerFuncWithNext`.

```go
package main

import (
	"net/http"

	"github.com/codegangsta/negroni"
	"github.com/duggan/go-torblock"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("Hello world!"))
	})

	t := torblock.New(torblock.Options{})
	t.Run()

	n := negroni.Classic()
	n.Use(negroni.HandlerFunc(t.HandlerFuncWithNext))
	n.UseHandler(mux)

	n.Run("0.0.0.0:3000")
}

```

## TODO

* Tests, code documentation... that sort of thing.