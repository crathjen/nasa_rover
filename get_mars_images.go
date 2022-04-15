package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/tidwall/gjson"
)

func main() {

	// should be made configurable in cli tool
	maxImages := 3
	dayLookback := 10
	rover := "curiosity"
	camera := "NAVCAM"

	client := &NASAClientImpl{
		//wildly insecure maybe pull it in from shell environment
		Api_Key:    "DEMO_KEY",
		Cache: &ImageCacheNoopImpl{},
	}

	imageCollection, err := FetchImages(client, maxImages, dayLookback, rover, camera)
	if err != nil {
		fmt.Printf("error encountered: %s", err.Error())
		os.Exit(1)
	}

	bytes, err := json.Marshal(imageCollection)
	if err != nil {
		fmt.Printf("error encountered: %s", err.Error())
		os.Exit(2)
	}

	os.Stdout.Write(bytes)

}

const ISODate = "2006-01-02"

func FetchImages(client NASAClient, maxImages, dayLookback int, rover, camera string) (map[string][]string, error) {
	now := time.Now()
	imagesMap := make(map[string][]string)
	for i := 0; i < dayLookback; i++ {
		earthDate := now.AddDate(0, 0, -i).Format(ISODate)
		images, err := client.GetImages(ImageRequest{Camera: camera, Rover: rover, EarthDate: earthDate, MaxImages: maxImages})
		if err != nil {
			return nil, err
		}
		imagesMap[earthDate] = images
	}
	return imagesMap, nil
}

type NASAClient interface {
	GetImages(r ImageRequest) ([]string, error)
}

type ImageRequest struct {
	Camera    string
	Rover     string
	EarthDate string
	MaxImages int
}

type NASAClientImpl struct {
	Api_Key string
	Client  http.Client
	Cache   ImageCache
}

//https://api.nasa.gov/mars-photos/api/v1/rovers/curiosity/photos?earth_date=2016-4-2&camera=NAVCAM&api_key=DEMO_KEY

func (c *NASAClientImpl) getURL(r ImageRequest) string {
	URL := url.URL{Scheme: "https", Host: "api.nasa.gov", Path: fmt.Sprintf("mars-photos/api/v1/rovers/%v/photos", r.Rover)}
	q := URL.Query()
	q.Set("camera", r.Camera)
	q.Set("earth_date", r.EarthDate)
	q.Set("api_key", c.Api_Key)
	URL.RawQuery = q.Encode()
	return URL.String()
}

func (c *NASAClientImpl) GetImages(r ImageRequest) ([]string, error) {
	cachedValue, found := c.Cache.Get(r)
	if found {
		return cachedValue, nil
	} else {
		resp, err := c.Client.Get(c.getURL(r))
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("bad response code: %d", resp.StatusCode)
		}
		defer resp.Body.Close()
		bodyBytes, err := io.ReadAll(resp.Body) 
		if err != nil {
			return nil, err
		}

		images := gjson.GetBytes(bodyBytes, "photos.#.img_src")
		
		imageStrings := make([]string, 0)
		count := 0
		images.ForEach(func(_, value gjson.Result) bool {
			imageStrings = append(imageStrings, value.Str)
			count ++
			if count == r.MaxImages {
				return false
			}else {
				return true
			}
		})
		return imageStrings, nil
	}
}

type ImageCache interface {
	Put(key ImageRequest, value []string)
	Get(key ImageRequest) ([]string, bool)
}

type ImageCacheNoopImpl struct{}

func (ic *ImageCacheNoopImpl) Put(key ImageRequest, value []string) {

}
func (ic *ImageCacheNoopImpl) Get(key ImageRequest) ([]string, bool) {
	return nil, false
}
