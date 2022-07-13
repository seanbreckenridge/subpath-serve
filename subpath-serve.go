package main

import (
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// default port to serve subpath-serve on
const defaultPort = 8050

// paths to ignore from serveFolder
var ignorePaths = [...]string{".git"}

// configuration information
type config struct {
	port        int
	serveFolder string
	repoPrefix  string
}

// PageLines is used for the Index page
// which needs each line to be split up so links can be added
// If PageLines is empty, uses pageContents instead
type PageInfo struct {
	Title        string
	PageContents string
	PageLines    []string
	PrefixInfo   *HttpPrefix
}

type HttpPrefix struct {
	Url      string
	Hostname string
}

func parseFlags() *config {
	// flag definitions
	port := flag.Int("port", 8050, "port to serve subpath-serve on")
	serveFolder := flag.String("folder", "./serve", "path to serve subpath-serve on")
	repoPrefix := flag.String("git-http-prefix", "", "Optionally, provide a prefix which when the matched filepath is appended to, links to a git web view (e.g. https://github.com/seanbreckenridge/dotfiles/blob/master)")
	// print repo in help text
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: subpath-serve [FLAG...]\nFor instructions, see https://github.com/seanbreckenridge/subpath-serve")
		fmt.Fprintln(os.Stderr, "")
		flag.PrintDefaults()
	}
	// parse flags
	flag.Parse()
	// make sure path is valid
	fileInfo, err := os.Stat(*serveFolder)
	if err != nil {
		log.Fatalf("Error: Folder to serve files from, '%s' does not exist\n", *serveFolder)
	}
	if !fileInfo.IsDir() {
		log.Fatalf("Error: Path '%s' is not a directory", *serveFolder)
	}
	return &config{
		port:        *port,
		serveFolder: *serveFolder,
		repoPrefix:  strings.TrimSpace(*repoPrefix),
	}
}

func setupTemplate() *template.Template {
	tmpl, err := template.New("dark").Parse(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><style>
html, body {
         margin: 0px;
         padding: 0px;
         border: 0px;
         width: 100%;
         min-height: 100vh;
         background-color: #111;
         color: white;
         font-family: "Courier", sans-serif;
     }
     main {
         display: flex;
         justify-content: center;
     }
     .container {
         width: 90%;
         margin: 2rem;
     }
     div#rounded {
         background-color: #1d2330;
         font-size: 120%;
         margin: 1rem;
         padding: 1rem;
         border-radius: min(0.25rem, 15px);
     }
     .title {
         display: flex;
         flex-direction: row;
         justify-content: flex-end;
         width: 90%;
         margin-left: auto;
         margin-right: auto;
     }
     code {
         white-space: pre-wrap; /* css-3 */
         white-space: -moz-pre-wrap; /* Mozilla, since 1999 */
         white-space: -pre-wrap; /* Opera 4-6 */
         white-space: -o-pre-wrap; /* Opera 7 */
         word-wrap: break-word; /* Internet Explorer 5.5+ */
     }
     p {
         margin: 4px;
     }
     a {
         color: #0779e4;
     }
     a:visited {
         color: #4cbbb9;
     }
     a:hover {
          color: #77d8d8;
     }
     a:active {
         color: #eff3c6;
     }
     footer {
         display: flex;
         flex-direction: column;
         justify-content: flex-start;
         width: 80%;
         margin-left: auto;
         margin-right: auto;
         padding-bottom: 1rem;
     }
     footer div {
         padding-top: 0.5rem;
         padding-bottom: 0.5rem;
    }
    </style>
    <title>{{ .Title }}</title>
</head>
<body>
    <main>
        <div class="container">
            <div class="title">
                <a href="#" onclick="RawFile()">Raw</a>
            </div>
            <div id="rounded">
{{ range $element := .PageLines }}
<p><a href="./{{ $element }}?dark">{{ $element }}</a></p>
{{ else }}<pre><code>{{ .PageContents }}</code></pre>{{ end }}
            </div>
        </div>
    </main>

    <footer>
				{{ if .PrefixInfo  }}
				<div>View on <a href="{{ .PrefixInfo.Url }}">{{ .PrefixInfo.Hostname }}</a></div>
				{{ end }}
        <div>Served with <a href="https://github.com/seanbreckenridge/subpath-serve">subpath-serve</a></div>
    </footer>
    <script>
        function RawFile() {
            window.location.href = window.location.href.substring(0, window.location.href.lastIndexOf("?"));
        }
    </script>
</body>
</html>
`)
	if err != nil {
		panic(err)
	}
	return tmpl
}

// generates the response for the "/" request
func index() string {
	var indexBuilder strings.Builder
	err := filepath.Walk(".",
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			// if the filename matches any of the paths in the global ignorePaths
			// skip the directory
			for _, ignore := range ignorePaths {
				if info.Name() == ignore {
					return filepath.SkipDir
				}
			}
			if path != "." {
				// if this is a file
				if info.Mode().IsRegular() {
					// else append to response string
					indexBuilder.WriteString(path)
					indexBuilder.WriteString("\n")
				}
			}
			return nil
		})
	if err != nil {
		panic(err)
	}
	return indexBuilder.String()
}

// returns nil if file could not be found
// else, returns the contents of the file
//
// errors signify an application error (should be converted to 500)
func find(query string) (*string, error) {
	var foundPath *string
	err := filepath.Walk(".",
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			// if the filename matches any of the paths in the global ignorePaths
			// skip the directory
			for _, ignore := range ignorePaths {
				if info.Name() == ignore {
					return filepath.SkipDir
				}
			}
			if path != "." {
				// if this is a file
				if info.Mode().IsRegular() {
					// the query matches this path
					if strings.HasSuffix(path, query) &&
						query[strings.LastIndex(query, "/")+1:] == info.Name() {
						// if this matches the suffix of the file
						// return the filename
						foundPath = &path
						// return error from os.Walk func to exit once we find file
						return errors.New("early exit os.Walk")
					}
				}
			}
			return nil
		})
	// if os.walk error and not the early exit
	// return the error, since some os error actually happened
	if err != nil && err.Error() != "early exit os.Walk" {
		return nil, err
	}

	// return the filepath/nil if no file was found
	return foundPath, nil
}

// is dark req specifies whether or not this is a
// plain text response or rendered dark response
func render(w *http.ResponseWriter, info *PageInfo, tmpl *template.Template, isDarkReq bool) {
	if isDarkReq {
		tmpl.Execute(*w, *info)
	} else {
		fmt.Fprintf(*w, "%s", (*info).PageContents)
	}
}

// https://github.com/seanbreckenridge/dotfiles/blob/master -> github.com
func getDomainName(httpPrefixUrl string) string {
	name := "repository"
	if httpPrefixUrl != "" {
		u, err := url.Parse(httpPrefixUrl)
		if err == nil {
			parts := strings.Split(u.Hostname(), ".")
			return parts[len(parts)-2]
		}
	}
	return name
}

func hasQueryParam(queryValues url.Values, queryParam string) bool {
	_, ok := queryValues[queryParam]
	return ok
}

func main() {
	config := parseFlags()
	tmpl := setupTemplate()
	err := os.Chdir(config.serveFolder)
	if err != nil {
		panic(err)
	}
	httpPrefixName := strings.Title(getDomainName(config.repoPrefix))
	// global handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		queryParams := r.URL.Query()
		isDark := hasQueryParam(queryParams, "dark")
		isRedirect := hasQueryParam(queryParams, "redirect")
		r.URL.Query()
		if r.URL.Path == "/" {
			// split the content into multiple lines if this is a html response
			// so that links can be added nicely
			pageContents := index()
			pageLines := []string{}
			if isDark {
				pageLines = strings.Split(strings.Trim(pageContents, "\n"), "\n")
			}
			render(&w, &PageInfo{
				PageContents: pageContents,
				Title:        "Index",
				PageLines:    pageLines,
			}, tmpl, isDark)
		} else {
			// search for the file
			foundPath, err := find(strings.TrimRight(r.URL.Path[1:], "/"))
			// if there was an OS error
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				render(&w, &PageInfo{
					PageContents: err.Error(),
					Title:        "Server Error",
				}, tmpl, isDark)
			} else {
				// if the file couldnt be found
				if foundPath == nil {
					w.WriteHeader(http.StatusNotFound)
					render(&w, &PageInfo{
						PageContents: fmt.Sprintf("Could not find a match for %s\n", r.URL.Path[1:]),
						Title:        "404 - Not Found",
					}, tmpl, isDark)
					return
				}
				// file was found
				url := fmt.Sprintf("%s/%s", config.repoPrefix, *foundPath)
				// if were meant to redirect, early return
				if isRedirect {
					if config.repoPrefix != "" {
						http.Redirect(w, r, url, 302)
						return
					}
					fmt.Fprintf(os.Stderr, "Warning: tried to redirect to %s but no repoPrefix set\n", url)
				}
				// if the file was found, return the read file
				data, _ := ioutil.ReadFile(*foundPath)
				w.Header().Set("X-Filepath", *foundPath)
				render(&w, &PageInfo{
					PageContents: string(data),
					Title:        *foundPath,
					PrefixInfo: &HttpPrefix{
						Url:      url,
						Hostname: httpPrefixName,
					},
				}, tmpl, isDark)
			}
		}
	})
	log.Printf("subpath-serve serving %s on port %d\n", config.serveFolder, config.port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", config.port), nil))
}
