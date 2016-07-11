package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"goji.io"
	"goji.io/pat"
	"goji.io/pattern"
	"golang.org/x/net/context"

	"github.com/tealoid/YoutubeExercise/private"
)

type pageInfo struct {
	TotalResults   int `json:"totalResults"`
	ResultsPerPage int `json:"resultsPerPage"`
}

type item struct {
	ID      map[string]string `json:"id"`
	Snippet snippet           `json:"snippet"`
}

type snippet struct {
	PublishedAt  string               `json:"publishedAt"`
	ChannelID    string               `json:"channelId"`
	Title        string               `json:"title"`
	Description  string               `json:"description"`
	Thumbnails   map[string]thumbnail `json:"thumbnails"`
	ChannelTitle string               `json:"channelTitle"`
}

type thumbnail struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type respType struct {
	Kind          string   `json:"kind"`
	Etag          string   `json:"etag"`
	NextPageToken string   `json:"nextPageToken"`
	PageInfo      pageInfo `json:"pageInfo"`
	RegionCode    string   `json:"regionCode"`
	Items         []item   `json:"items"`
}

type ytPageItem struct {
	Link        string
	Title       string
	Description string
	Image       string
}

const youtubeAPIURL = "https://www.googleapis.com/youtube/v3/search?part=snippet&order=relevance&key="

func main() {
	mux := goji.NewMux()
	params := goji.SubMux()

	mux.HandleC(pat.New("/yt/*"), params)
	params.HandleFuncC(pat.Get("/:q"), handleQuery)
	params.HandleFuncC(pat.Get("/:q/:type"), handleQuery)
	params.HandleFuncC(pat.Get("/:q/:type/:max"), handleQuery)
	params.UseC(middleware) //We can apply context aware middleware to Mux like this.
	http.ListenAndServe(":9996", mux)

}

//This does nothing important. Just added for giggles.
func middleware(h goji.Handler) goji.Handler {
	t := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		fmt.Println("Request URI: ", r.RequestURI)
		fmt.Println("Context Values: ", ctx.Value(pattern.AllVariables))
		h.ServeHTTPC(ctx, w, r)
	}
	return goji.HandlerFunc(t)
}

func handleQuery(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	var q string
	t := "video"
	maxResults := "10"

	//Easier and works if we know the amount of parameters we will get.
	//Causes panic if the parameter is not found.
	//t := pat.Param(ctx, "type")
	//q := pat.Param(ctx, "q")
	//maxResults := pat.Param(ctx, "max")

	if val := ctx.Value(pattern.Variable("q")); val != nil {
		q = val.(string)
	}
	if val := ctx.Value(pattern.Variable("max")); val != nil {
		if res, err := strconv.Atoi(val.(string)); err != nil {
			fmt.Println("Conversion error: ", err)
		} else if res > 50 {
			maxResults = "50"
		} else {
			maxResults = val.(string)
		}
	}
	if val := ctx.Value(pattern.Variable("type")); val != nil {
		//This if loop could be combined with the parent but its clearer this way.
		if val == "video" || val == "channel" || val == "user" {
			t = val.(string)
		}

	}

	fmt.Println(fmt.Sprintf("Handled params [ q: %s, type: %s, maxResults: %s %s", q, t, maxResults, "]"))

	query := queryBuilder(q, t, maxResults)

	resp, err := http.Get(query)
	defer resp.Body.Close()
	if err != nil {
		fmt.Println(err)
		w.Write([]byte("Sorry, error."))
		return
	}

	respBytes, _ := ioutil.ReadAll(resp.Body)

	var data respType
	json.Unmarshal(respBytes, &data)

	if data.PageInfo.TotalResults == 0 {
		fmt.Println("0 results found!")
		w.Write([]byte("0 results found!"))
		return
	}

	pageItems := make([]ytPageItem, 0, 10)
	for _, i := range data.Items {
		pageItems = append(pageItems, handleItem(i))
	}
	servePage(w, pageItems, data.PageInfo)
}

func servePage(w http.ResponseWriter, pageItems []ytPageItem, pInfo pageInfo) {
	var response bytes.Buffer
	response.WriteString(`<html><body><ul style="list-style-type:none; width: 800px">`)
	response.WriteString(fmt.Sprintf(`<li style="margin-bottom: 40px; font-weight: bold"><p>Found %d results! Displaying %d.</p></li>`, pInfo.TotalResults, pInfo.ResultsPerPage))
	for _, yt := range pageItems {
		response.WriteString(fmt.Sprintf(
			`<li style="margin-bottom: 40px">
				<ul style="list-style-type:none">
					<li>
						%s    %s
					</li>
					<li>
						%s
					</li>
				</ul>
			</li>`,
			yt.Image, yt.Title, yt.Description))
	}
	response.WriteString("</ul></body></html>")
	w.Write(response.Bytes())
}

func handleItem(i item) ytPageItem {
	//"kind" is a string youtube#{item type}, so cut youtube#.
	tag := i.ID["kind"][8:]
	var link string

	switch tag {
	case "video":
		link = fmt.Sprintf(`http://youtube.com/watch?v=%s`, i.ID["videoId"])

	case "channel":
		link = fmt.Sprintf(`http://youtube.com/channel/%s`, i.ID["channelId"])

	default:
		link = "http://youtube.com"
	}
	return createPageItem(i, link)
}

func createPageItem(i item, link string) ytPageItem {
	var pageItem ytPageItem
	pageItem.Title = fmt.Sprintf(`<a href="%s"><h3>%s</h3></a>`, link, i.Snippet.Title)
	pageItem.Description = fmt.Sprintf(`<p style="word-wrap: break-word;">%s</p>`, i.Snippet.Description)
	pageItem.Image = fmt.Sprintf(`<a href="%s"><Img src="%s" style="width: %d; height: %d;" /></a>`, link, i.Snippet.Thumbnails["default"].URL, 120, 100)
	return pageItem
}

func queryBuilder(q, t, maxResults string) string {
	var buffer bytes.Buffer

	if _, err := strconv.Atoi(maxResults); err != nil {
		maxResults = "10"
	}
	buffer.WriteString(fmt.Sprintf("%s%s", youtubeAPIURL, private.DevAPIKey))
	buffer.WriteString(fmt.Sprintf("&q=%s", url.QueryEscape(q)))
	buffer.WriteString(fmt.Sprintf("&type=%s", url.QueryEscape(t)))
	buffer.WriteString(fmt.Sprintf("&maxResults=%s", url.QueryEscape(maxResults)))

	fmt.Println(buffer.String())
	return buffer.String()
}
