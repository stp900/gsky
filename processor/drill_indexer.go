package processor

import (
	geo "bitbucket.org/monkeyforecaster/geometry"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// string used to format Go ISO times
const ISOFormat = "2006-01-02T15:04:05.000Z"

type FileList struct {
	Files []string `json:"files"`
}

type DrillIndexer struct {
	Context    context.Context
	In         chan *GeoDrillRequest
	Out        chan *GeoDrillGranule
	Error      chan error
	APIAddress string
}

//func NewDrillIndexer(apiAddr string, errChan chan error) (*DrillIndexer) {
func NewDrillIndexer(ctx context.Context, apiAddr string, errChan chan error) *DrillIndexer {
	return &DrillIndexer{
		Context:    ctx,
		In:         make(chan *GeoDrillRequest, 100),
		Out:        make(chan *GeoDrillGranule, 100),
		Error:      errChan,
		APIAddress: apiAddr,
	}
}

func (p *DrillIndexer) Run() {
	defer close(p.Out)
	for geoReq := range p.In {
		var feat geo.Feature
		err := json.Unmarshal([]byte(geoReq.Geometry), &feat)
		if err != nil {
			p.Error <- fmt.Errorf("Problem unmarshalling GeoJSON object: %v", geoReq.Geometry)
			return
		}

		start := time.Now()
		for _, nameSpace := range geoReq.NameSpaces {
			reqURL := strings.Replace(fmt.Sprintf("http://%s%s?intersects&metadata=gdal&time=%s&until=%s&srs=%s&namespace=%s", p.APIAddress, geoReq.Collection, geoReq.StartTime.Format(ISOFormat), geoReq.EndTime.Format(ISOFormat), geoReq.CRS, nameSpace), " ", "%20", -1)
			featWKT := feat.Geometry.MarshalWKT()
			resp, err := http.PostForm(reqURL, url.Values{"wkt": {featWKT}})
			if err != nil {
				p.Error <- fmt.Errorf("POST request to %s failed. Error: %v", reqURL, err)
				continue
			}

			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				p.Error <- fmt.Errorf("Error parsing response body from %s. Error: %v", reqURL, err)
				continue
			}

			var metadata MetadataResponse
			err = json.Unmarshal(body, &metadata)
			if err != nil {
				fmt.Println(string(body))
				p.Error <- fmt.Errorf("Problem parsing JSON response from %s. Error: %v", reqURL, err)
				continue
			}

			switch len(metadata.GDALDatasets) {
			case 0:
				p.Out <- &GeoDrillGranule{"NULL", nameSpace, "Byte", nil, geoReq.Geometry, geoReq.CRS}
			default:
				for _, ds := range metadata.GDALDatasets {
					p.Out <- &GeoDrillGranule{ds.DSName, nameSpace, ds.ArrayType, ds.TimeStamps, geoReq.Geometry, geoReq.CRS}
				}
			}
		}
		log.Println("Indexer Time Total", time.Since(start))
	}
}
