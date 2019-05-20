package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/grafov/m3u8"
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

type playlistServer struct {
	Config      AppConfig
	Playlists   *PlaylistService
	Balancer    LoadBalancer
	ProxyLoader *ProxyLoader
}

func (p *playlistServer) HandlerFunc() http.HandlerFunc {
	router := mux.NewRouter()
	router.HandleFunc("/hls/{channel}.m3u8", p.CORSHandlerFunc(p.HandleMasterPlaylist)).Methods("GET")
	router.HandleFunc("/hls/{channel}.m3u8", p.CORSHandlerFunc(p.HandleOptions)).Methods("OPTIONS")
	router.HandleFunc("/hls/{channel}/index.m3u8", p.CORSHandlerFunc(p.HandleMediaPlaylist)).Methods("GET")
	router.HandleFunc("/hls/{channel}/index.m3u8", p.CORSHandlerFunc(p.HandleOptions)).Methods("OPTIONS")

	return router.ServeHTTP
}

func (p *playlistServer) HandleOptions(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(200)
	res.Write([]byte{})
}

func (p *playlistServer) HandleMasterPlaylist(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	playlist, err := p.Playlists.Load(vars["channel"], m3u8.MASTER)
	if err != nil {
		serveError(res, req, 503, err)
		return
	}

	servePlaylist(res, req, playlist)
}

func (p *playlistServer) HandleMediaPlaylist(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	channel := vars["channel"]

	playlist, err := p.Playlists.Load(channel, m3u8.MEDIA)
	if err != nil {
		serveError(res, req, 503, err)
		return
	}

	sessionKey := getRequestIP(req, p.Config.IPHeaderName)
	playlist, err = RewriteMediaPlaylist(playlist.(*m3u8.MediaPlaylist), sessionKey, channel, p.Balancer)
	if err != nil {
		serveError(res, req, 500, err)
		return
	}

	servePlaylist(res, req, playlist)
}

func getRequestIP(req *http.Request, headerName string) string {
	if headerName == "" {
		ip, _, _ := net.SplitHostPort(req.RemoteAddr)
		return ip
	}
	return req.Header.Get(headerName)
}

func serveError(res http.ResponseWriter, req *http.Request, code int, err error) {
	res.WriteHeader(code)
	res.Write([]byte(err.Error()))
}

func servePlaylist(res http.ResponseWriter, req *http.Request, playlist m3u8.Playlist) {
	buf := playlist.Encode()

	res.Header().Add("Content-Type", "application/vnd.apple.mpegurl")
	res.Header().Set("Content-Length", strconv.FormatInt(int64(buf.Len()), 10))
	res.WriteHeader(200)
	res.Write(buf.Bytes())
}

func (p *playlistServer) CORSHandlerFunc(handler http.HandlerFunc) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		if len(p.Config.CORSOrigins) == 0 {
			res.Header().Add("Access-Control-Allow-Origin", "*")
		} else {
			for _, origin := range p.Config.CORSOrigins {
				if req.Header.Get("Origin") == origin {
					res.Header().Add("Access-Control-Allow-Origin", origin)
					break
				}
			}
		}

		res.Header().Add("Access-Control-Allow-Headers", "Content-Type,Origin")
		res.Header().Add("Access-Control-Allow-Methods", "GET,OPTIONS")
		res.Header().Add("Access-Control-Max-Age", "86400")

		handler(res, req)
	}
}

func main() {
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	var config AppConfig
	if err := ReadYAMLConfig(configPath, &config); err != nil {
		log.Fatalf("error reading config: %v", err)
	}

	proxyEvents := make(chan *ProxyStatusEvent)
	proxyLoader := NewProxyLoader(config.ProxyLoader)
	proxyLoader.Notify(proxyEvents)
	go proxyLoader.Start()

	var balancerConfig ConsistentHashBalancerConfig
	readProxyConfig(&balancerConfig)
	balancer := NewConsistentHashBalancer(balancerConfig, proxyEvents)

	server := playlistServer{
		Config: config,
		Playlists: NewPlaylistService(
			&PlaylistLoader{
				UpstreamServers: config.UpstreamServers,
			},
			config.CacheTTL,
		),
		Balancer:    balancer,
		ProxyLoader: proxyLoader,
	}

	log.Printf("listening on: %s", config.HTTPAddress)

	httpServer := &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		Addr:         config.HTTPAddress,
		Handler:      server.HandlerFunc(),
	}

	log.Fatal(httpServer.ListenAndServe())
}
