package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

var configPath string

func init() {
	flag.StringVar(&configPath, "config", "", "config file path")
}

func readProxyConfig(config interface{}) {
	if err := ReadYAMLConfig(configPath, config); err != nil {
		log.Fatalf("error reading proxy config: %v", err)
	}
}

func main() {
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	var config AppConfig
	if err := ReadYAMLConfig(configPath, &config); err != nil {
		log.Fatalf("error reading config: %v", err)
	}

	var playlistBalancer LoadBalancer
	switch config.ProxyStrategy {
	case "WEIGHTED_RANDOM":
		var balancerConfig WeightedRandomBalancerConfig
		readProxyConfig(&balancerConfig)
		playlistBalancer = NewWeightedRandomBalancer(balancerConfig)

	case "ROUND_ROBIN":
		var balancerConfig RoundRobinBalancerConfig
		readProxyConfig(&balancerConfig)
		playlistBalancer = NewRoundRobinBalancer(balancerConfig)

	default:
		log.Fatalf("invalid proxy strategy: %s", config.ProxyStrategy)
	}

	playlists := NewPlaylistService(&PlaylistLoader{
		UpstreamServers: config.UpstreamServers,
	})

	handleReport := func(res http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		channel := vars["channel"]

		playlist, err := playlists.Load(channel)
		if err != nil {
			log.Printf("error loading channel %s: %v", channel, err)

			res.WriteHeader(503)
			res.Write([]byte(err.Error()))
			return
		}

		lastModified := playlist.LastModified()

		ifModifiedSinceHeader := req.Header.Get("If-Modified-Since")
		ifModifiedSince, err := time.Parse(time.RFC1123, ifModifiedSinceHeader)
		if err == nil && !lastModified.After(ifModifiedSince) {
			res.WriteHeader(304)
			return
		}

		routedPlaylist, err := playlist.Route(playlistBalancer)
		if err != nil {
			res.WriteHeader(500)
			res.Write([]byte(err.Error()))
			return
		}

		buf := routedPlaylist.Encode()

		res.Header().Add("Last-Modified", lastModified.Format(time.RFC1123))
		res.Header().Add("Max-Age", strconv.FormatInt(int64(playlist.TargetDuration()/time.Second), 10))
		res.Header().Add("Content-Type", "application/vnd.apple.mpegurl")
		res.Header().Set("Content-Length", strconv.FormatInt(int64(buf.Len()), 10))
		res.WriteHeader(200)
		res.Write(buf.Bytes())
	}

	router := mux.NewRouter()
	router.HandleFunc("/hls/{channel}/index.m3u8", handleReport).Methods("GET")

	corsHeaders := handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Origin"})
	corsMaxAge := handlers.MaxAge(86400)
	corsOrigins := handlers.AllowedOrigins([]string{"*"})
	corsMethods := handlers.AllowedMethods([]string{"GET", "OPTIONS"})
	corsMiddleware := handlers.CORS(corsHeaders, corsMaxAge, corsOrigins, corsMethods)

	log.Printf("listening on: %s", config.HTTPAddress)
	log.Fatal(http.ListenAndServe(config.HTTPAddress, corsMiddleware(router)))
}
