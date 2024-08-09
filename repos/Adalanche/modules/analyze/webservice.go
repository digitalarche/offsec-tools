package analyze

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/gin-contrib/pprof"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	jsoniter "github.com/json-iterator/go"
	"github.com/lkarlslund/adalanche/modules/engine"
	"github.com/lkarlslund/adalanche/modules/ui"
)

//go:embed html/*
var embeddedassets embed.FS

var (
	qjson = jsoniter.ConfigCompatibleWithStandardLibrary
)

type UnionFS struct {
	filesystems []http.FileSystem
}

func (ufs *UnionFS) AddFS(newfs http.FileSystem) {
	ufs.filesystems = append(ufs.filesystems, newfs)
}

func (ufs UnionFS) Open(filename string) (http.File, error) {
	for _, fs := range ufs.filesystems {
		if f, err := fs.Open(filename); err == nil {
			return f, nil
		}
	}
	return nil, os.ErrNotExist
}

func (ufs UnionFS) Exists(prefix, filename string) bool {
	_, err := ufs.Open(filename)
	return err != os.ErrNotExist
}

type handlerfunc func(*engine.Objects, http.ResponseWriter, *http.Request)

type webservice struct {
	quit   chan bool
	Router *gin.Engine
	UnionFS
	Objs *engine.Objects
	srv  *http.Server

	AdditionalHeaders []string // Additional things to add to the main page
}

func NewWebservice() *webservice {
	ws := &webservice{
		quit:   make(chan bool),
		Router: gin.New(),
	}

	gin.SetMode(gin.ReleaseMode)

	ws.Router.Use(func(c *gin.Context) {
		start := time.Now() // Start timer
		path := c.Request.URL.Path

		// Process request
		c.Next()

		logger := ui.Info()
		if c.Writer.Status() >= 500 {
			logger = ui.Error()
		}

		logger.Msgf("%s %s (%v) %v, %v bytes", c.Request.Method, path, c.Writer.Status(), time.Since(start), c.Writer.Size())
	})
	ws.Router.Use(gin.Recovery()) // adds the default recovery middleware

	htmlFs, _ := fs.Sub(embeddedassets, "html")
	ws.AddFS(http.FS(htmlFs))

	// Add stock functions
	analysisfuncs(ws)

	// Add debug functions
	if ui.GetLoglevel() >= ui.LevelDebug {
		debugfuncs(ws)
	}

	return ws
}

func (w *webservice) QuitChan() <-chan bool {
	return w.quit
}

func (w *webservice) Start(bind string, objs *engine.Objects, localhtml []string) error {
	w.Objs = objs

	// Profiling
	pprof.Register(w.Router)

	w.srv = &http.Server{
		Addr:    bind,
		Handler: w.Router,
	}

	if len(localhtml) != 0 {
		w.UnionFS = UnionFS{}
		for _, html := range localhtml {
			// Override embedded HTML if asked to
			if stat, err := os.Stat(html); err == nil && stat.IsDir() {
				// Use local files if they exist
				ui.Info().Msgf("Adding local HTML folder %v", html)
				w.AddFS(http.FS(os.DirFS(html)))
			} else {
				ui.Fatal().Msgf("Could not add local HTML folder %v, failure: %v", html, err)
			}
		}
	}

	w.Router.GET("/", func(c *gin.Context) {
		indexfile, err := w.UnionFS.Open("index.html")
		if err != nil {
			ui.Error().Msgf("Could not open index.html: %v", err)
		}
		rawindex, _ := io.ReadAll(indexfile)
		indextemplate := template.Must(template.New("index").Parse(string(rawindex)))

		err = indextemplate.Execute(c.Writer, struct {
			AdditionalHeaders []string
		}{
			AdditionalHeaders: w.AdditionalHeaders,
		})
		if err != nil {
			ui.Error().Msgf("Could not render template index.html: %v", err)
		}
	})
	w.Router.Use(static.Serve("/", w.UnionFS))

	// w.Router.StaticFS("/", http.FS(w.UnionFS))

	go func() {
		if err := w.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			ui.Fatal().Msgf("Problem launching webservice listener: %s", err)
		}
	}()

	ui.Info().Msgf("Listening - navigate to http://%v/ ... (ctrl-c or similar to quit)", bind)

	return nil
}

func (w *webservice) ServeTemplate(c *gin.Context, path string, data any) {
	templatefile, err := w.UnionFS.Open(path)
	if err != nil {
		ui.Fatal().Msgf("Could not open template %v: %v", path, err)
	}
	rawtemplate, _ := io.ReadAll(templatefile)
	template, err := template.New(path).Parse(string(rawtemplate))
	if err != nil {
		c.AbortWithError(500, err)
		return
	}
	template.Execute(c.Writer, data)
}
