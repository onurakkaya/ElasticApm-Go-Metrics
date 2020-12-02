package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"elasticapm-demo/news"
	"github.com/joho/godotenv"

	// Required libs
	"go.elastic.co/apm"
	"go.elastic.co/apm/module/apmhttp"
	"go.elastic.co/apm/transport"
)

var tmpl = template.Must(template.ParseFiles("index.html"))
var clientTracer = apmhttp.WrapClient(http.DefaultClient)
var globalTracer = apm.DefaultTracer

type Search struct {
	Query      string
	NextPage   int
	TotalPages int
	Results    *news.Results
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	buf := &bytes.Buffer{}
	err := tmpl.Execute(buf, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	buf.WriteTo(w)
}

func searchHandler(newsapi *news.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := url.Parse(r.URL.String())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// HttpClient context -> it's covers current transaction obj. reference
		ctx := r.Context()
		apm.DefaultTracer.CaptureHTTPRequestBody(r)

		params := u.Query()
		searchQuery := params.Get("q")
		page := params.Get("page")
		if page == "" {
			page = "1"
		}
		// start span on current transaction and return the reference of span
		parentSpan, ctx := apm.StartSpan(ctx, searchQuery, "NewsAPI")
		parentSpan.Action = "FetchAPI"

		results, err := newsapi.FetchEverything(searchQuery, page)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			apm.CaptureError(ctx, err)
			return
		}

		resultString := fmt.Sprintf("%+v", results)

		spanOptions := apm.SpanOptions{
			Start:  time.Now(),
			Parent: parentSpan.TraceContext(),
		}

		childSpan := apm.DefaultTracer.StartSpan(resultString, "NewsApi", parentSpan.TraceContext().Span, spanOptions)
		childSpan.SetStacktrace(0)

		childSpan.End()

		// fmt.Printf("%+v", results)

		parentSpan.End()

		nextPage, err := strconv.Atoi(page)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		parallelSpan, ctx := apm.StartSpan(ctx, "Calculate Pagination", "Search")
		parallelSpan.Action = "RenderResults"

		search := &Search{
			Query:      searchQuery,
			NextPage:   nextPage,
			TotalPages: int(math.Ceil(float64(results.TotalResults / newsapi.PageSize))),
			Results:    results,
		}

		if ok := !search.IsLastPage(); ok {
			search.NextPage++
		}

		parallelSpan.End()

		buf := &bytes.Buffer{}
		err = tmpl.Execute(buf, search)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		buf.WriteTo(w)

		fmt.Println("Search Query is: ", searchQuery)
		fmt.Println("Page is: ", page)

	}
}

func (s *Search) IsLastPage() bool {
	return s.NextPage >= s.TotalPages
}

func (s *Search) CurrentPage() int {
	if s.NextPage == 1 {
		return s.NextPage
	}

	return s.NextPage - 1
}

func (s *Search) PreviousPage() int {
	return s.CurrentPage() - 1
}

func main() {

	// Close Default Apm tracer. (It initializes before program start.)
	// apm.DefaultTracer.Close()

	// Load real time environment variables from .env file.
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env fil")
	}

	// Reload Environment variables to Elastic Apm.
	if _, err := transport.InitDefault(); err != nil {
		log.Fatal(err)
	}

	// Set ServiceName and ServiceVersion vars. from .env file then Run new Apm Tracer.
	serviceName := os.Getenv("ELASTIC_APM_SERVICE_NAME")
	serviceVersion := os.Getenv("ELASTIC_APM_SERVICE_VERSION")
	tracer, err := apm.NewTracer(serviceName, serviceVersion)
	apm.DefaultTracer = tracer
	apm.DefaultTracer.SetCaptureBody(apm.CaptureBodyAll)

	globalTracer = tracer

	if err != nil {
		log.Fatal(err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	apiKey := os.Getenv("NEWS_API_KEY")
	if apiKey == "" {
		log.Fatal("Env: apiKey must be set")
	}

	myClient := &http.Client{Timeout: 10 * time.Second}
	clientTracer = apmhttp.WrapClient(myClient)

	// transaction := tracer.StartTransaction("Web-Request", "Web")

	// transaction.End()
	newsapi := news.NewClient(myClient, apiKey, 20)

	fs := http.FileServer(http.Dir("assets"))

	mux := http.NewServeMux()
	mux.Handle("/assets/", http.StripPrefix("/assets/", fs))
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/search", searchHandler(newsapi))

	// Wrap http serve mux with apm
	http.ListenAndServe(":"+port, apmhttp.Wrap(mux, apmhttp.WithTracer(tracer)))

}
