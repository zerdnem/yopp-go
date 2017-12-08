package main

import (
	"crypto/md5"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

var thefilename string
var thefiledata map[string][]string
var templates *template.Template
var autoeraseafterdownload = false

func Exists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func LoadTemplates(pattern string) {
	templates = template.Must(template.ParseGlob(pattern))
}

func ExecuteTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	templates.ExecuteTemplate(w, tmpl, data)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	if len(params) == 1 {
		exists := Exists(thefilename)
		if !exists && len(thefiledata) == 0 {
			templates.ExecuteTemplate(w, "download.html", nil)
			return
		}

		url := "http://" + r.Host + thefilename

		timeout := time.Duration(5) * time.Second
		transport := &http.Transport{
			ResponseHeaderTimeout: timeout,
			Dial: func(network, addr string) (net.Conn, error) {
				return net.DialTimeout(network, addr, timeout)
			},
			DisableKeepAlives: true,
		}
		client := &http.Client{
			Transport: transport,
		}
		resp, err := client.Get(url)
		if err != nil {
			fmt.Println(err)
		}
		defer resp.Body.Close()

		filename := strings.Join(strings.Split(thefiledata["fname"][0], " "), "-")

		w.Header().Set("Content-Description", "File Transfer")
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment; filename="+filename)
		w.Header().Set("Expires", "0")
		w.Header().Set("Cache-Control", "must-revalidate")
		w.Header().Set("Pragma", "public")
		w.Header().Set("Content-Length", r.Header.Get("Content-Length"))
		w.Header().Set("Content-Type", r.Header.Get("Content-Type"))

		if autoeraseafterdownload {
			os.Remove("." + thefilename)
			log.Println(thefiledata["fname"][0] + " deleted!")
		}

		for k := range thefiledata {
			delete(thefiledata, k)
		}

		io.Copy(w, resp.Body)

	}
	templates.ExecuteTemplate(w, "index.html", nil)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		//this part will not be triggered
		crutime := time.Now().Unix()
		h := md5.New()
		io.WriteString(h, strconv.FormatInt(crutime, 10))
		token := fmt.Sprintf("%x", h.Sum(nil))

		t, _ := template.ParseFiles("index.html")
		t.Execute(w, token)
	} else {
		//im sure r.URL.Query() is the right way
		//to do here
		r.ParseMultipartForm(32 << 20)

		m := r.MultipartForm
		thefiledata = m.Value
		for _, v := range m.File {
			for _, f := range v {
				file, err := f.Open()
				if err != nil {
					fmt.Println(err)
					return
				}
				defer file.Close()

				fi, err := os.OpenFile("./test/"+f.Filename, os.O_WRONLY|os.O_CREATE, 0666)
				if err != nil {
					fmt.Println(err)
					return
				}

				defer fi.Close()
				io.Copy(fi, file)
				log.Println("file uploaded: ", f.Filename)

				thefilename = "/test/" + f.Filename

				data := []byte("DONE")
				w.Write(data)

			}
		}

	}
}

func main() {
	r := mux.NewRouter()
	fs := http.FileServer(http.Dir("./test/"))
	r.PathPrefix("/test/").Handler(http.StripPrefix("/test/", fs))

	r.HandleFunc("/", indexHandler).Methods("GET")
	r.HandleFunc("/", uploadHandler)

	LoadTemplates("templates/*.html")
	http.Handle("/", r)
	log.Println("Serving at localhost:8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
