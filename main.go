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

	case "CONSISTENT_HASH":
		var balancerConfig ConsistentHashBalancerConfig
		readProxyConfig(&balancerConfig)
		playlistBalancer = NewConsistentHashBalancer(balancerConfig)

	default:
		log.Fatalf("invalid proxy strategy: %s", config.ProxyStrategy)
	}

	playlists := NewPlaylistService(&PlaylistLoader{
		UpstreamServers: config.UpstreamServers,
	})

	handleIndex := func(res http.ResponseWriter, req *http.Request) {
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

		var sessionKey string
		if config.IPHeaderName == "" {
			sessionKey = req.RemoteAddr
		} else {
			sessionKey = req.Header.Get(config.IPHeaderName)
		}

		routedPlaylist, err := playlist.Route(sessionKey, playlistBalancer)
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
	router.HandleFunc("/hls/{channel}/index.m3u8", handleIndex).Methods("GET")

	log.Printf("listening on: %s", config.HTTPAddress)

	corsHeaders := handlers.AllowedHeaders([]string{"Content-Type", "Origin"})
	corsMaxAge := handlers.MaxAge(86400)

	var corsOrigins handlers.CORSOption
	if len(config.CORSOrigins) != 0 {
		corsOrigins = handlers.AllowedOrigins(config.CORSOrigins)
	} else {
		corsOrigins = handlers.AllowedOrigins([]string{"*"})
	}

	corsMethods := handlers.AllowedMethods([]string{"GET", "OPTIONS"})
	corsMiddleware := handlers.CORS(corsHeaders, corsMaxAge, corsOrigins, corsMethods)

	log.Fatal(http.ListenAndServe(config.HTTPAddress, corsMiddleware(router)))
}
