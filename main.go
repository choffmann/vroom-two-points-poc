package main

import (
	"context"
	"embed"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math"
	"net/http"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)

	server := Server{}
	go func() {
		defer wg.Done()
		if err := server.Run(ctx); err != nil {
			slog.Error("failed to start http server", "error", err)
		}
	}()

	wg.Wait()
}

type Point struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

func distance(p1, p2 Point) float64 {
	radlat1 := float64(math.Pi * p1.Lat / 180)
	radlat2 := float64(math.Pi * p2.Lat / 180)

	theta := float64(p1.Lng - p2.Lng)
	radtheta := float64(math.Pi * theta / 180)

	dist := math.Sin(radlat1)*math.Sin(radlat2) + math.Cos(radlat1)*math.Cos(radlat2)*math.Cos(radtheta)
	if dist > 1 {
		dist = 1
	}

	dist = math.Acos(dist)
	dist = dist * 180 / math.Pi
	dist = dist * 60 * 1.1515

	dist = dist * 1.609344 // map to kilometers

	return dist
}

var (
	startPoint  = Point{54.76849412606307, 9.434964189058697}
	endPoint    = Point{54.76849412606307, 9.434964189058697}
	waterPoints = []Point{
		{54.76849412606307, 9.434964189058697},
		{54.80539072138063, 9.449963237272696},
	}
)

type Server struct{}

func (s *Server) Run(ctx context.Context) error {
	r := chi.NewRouter()

	r.Get("/hello", s.handleHelloWorld)
	r.Post("/v1/nearest", s.handleNearest)
	r.Post("/v2/nearest", s.handleNearestV2)
	r.Get("/*", s.handleFileSystem)

	server := http.Server{
		Addr:    ":1123",
		Handler: r,
	}

	go func() {
		<-ctx.Done()
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := server.Shutdown(timeoutCtx); err != nil {
			slog.Error("failed to shutdown http server", "error", err)
		}
	}()

	return server.ListenAndServe()
}

func (s *Server) handleHelloWorld(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte("Hello World"))
}

//go:embed all:ui
var f embed.FS

func (s *Server) handleFileSystem(w http.ResponseWriter, r *http.Request) {
	fSub, err := fs.Sub(f, "ui")
	if err != nil {
		panic(err)
	}
	rctx := chi.RouteContext(r.Context())
	pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
	fs := http.StripPrefix(pathPrefix, http.FileServerFS(fSub))
	fs.ServeHTTP(w, r)
}

type NearestReq struct {
	Trees []Point `json:"trees"`
	Wp    []Point `json:"wp"`
}

type NearestResp struct {
	Tree Point `json:"tree"`
	Wp   Point `json:"wp"`
}

func (s *Server) handleNearest(w http.ResponseWriter, r *http.Request) {
	var req NearestReq

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	m := make([]NearestResp, len(req.Trees))
	for i, tree := range req.Trees {
		dis := 1.7e+308
		finalWP := Point{}
		for _, wp := range req.Wp {
			d := distance(tree, wp)
			fmt.Printf("distance form %v and %v is %v\n", tree, wp, d)
			if dis > d {
				fmt.Printf("old dis: %f, new dis: %f\n", dis, d)
				dis = d
				finalWP = wp
			}
		}
		m[i] = NearestResp{
			Tree: tree,
			Wp:   finalWP,
		}
	}

	w.Header().Add("Content-Type", "application/json")

	encode := json.NewEncoder(w)
	encode.Encode(m)
}

type ValhallaPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type ValhallaMatrix struct {
	Sources []ValhallaPoint `json:"sources"`
	Targets []ValhallaPoint `json:"targets"`
	Costing string          `json:"costing"`
}

func (s *Server) handleNearestV2(w http.ResponseWriter, r *http.Request) {
	var req NearestReq

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	reqBody := ValhallaMatrix{
		Sources: slices.Collect(Map(slices.Values(req.Wp), func(v Point) ValhallaPoint { return ValhallaPoint{Lat: v.Lat, Lon: v.Lng} })),
		Targets: slices.Collect(Map(slices.Values(req.Trees), func(t Point) ValhallaPoint { return ValhallaPoint{Lat: t.Lat, Lon: t.Lng} })),
		Costing: "auto",
	}

	var buf strings.Builder
	if err := json.NewEncoder(&buf).Encode(&reqBody); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	valhallaReq, err := http.NewRequest(http.MethodPost, "http://localhost:8002/sources_to_targets", http.NoBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	query := valhallaReq.URL.Query()
	query.Add("json", buf.String())
	valhallaReq.URL.RawQuery = query.Encode()

	resp, err := http.DefaultClient.Do(valhallaReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			slog.Error("error response status not ok", "status_code", resp.StatusCode, "body", body)
		} else {
			slog.Error("error response status not ok", "status_code", resp.StatusCode, "body", "error body parsing")
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.Write(body)
}
