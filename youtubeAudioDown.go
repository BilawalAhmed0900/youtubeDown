package main

import (
    "io"
    "os"
    "math"
    "fmt"
    "strings"
    "strconv"
    "io/ioutil"
    "net/http"
    "encoding/json"
)

/*
    "formats" section.
    Shown as qualities in our browser
    A list of json dictionery
    {
        "itag",
        "url",
        "mimeType"
        "bitrate",
        "width",
        "height",
        "lastModified",
        "contentLength",
        "quality",
        "qualityLabel",
        "projectionType",
        "averageBitrate",
        "audioQuality",
        "approxDurationMs",
        "audioSampleRate"
    }
*/
/*
    "adaptiveFormats" section.
    Indiviual files. Video and audio seperates
    "itag",
    "url",
    "mimeType"
    "bitrate",
    "width",
    "height",
    "initRange",
    "indexRange",
    "lastModified",
    "contentLength",
    "quality",
    "fps",
    "qualityLabel",
    "projectionType",
    "averageBitrate",
    "highReplication",
    "audioQuality",
    "approxDurationMs",
    "audioSampleRate"
*/

/*type Range struct {
    start string `json:"start"`
    end string `json:"end"`
}

type ColorInfo struct {
    Primiries string `json:"primaries"`
    TransferCharacteristics string `json:"transferCharacteristics"`
    MatrixCoefficients string `json:"matrixCoefficients"`
}*/

type adaptiveFormat struct {
    Itag int `json:"itag"`
    Url string `json:"url"`
    MimeType string `json:"mimeType"`
    Bitrate int `json:"bitrate"`
    // Width int `json:"width,omitempty"`
    // Height int `json:"height,omitempty"`
    // InitRange Range `json:"initRange,omitempty"`
    // IndexRange Range `json:"indexRange,omitempty"`
    // LastModified string `json:"lastModified,omitempty"`
    // ContentLength string `json:"contentLength,omitempty"`
    // Quality string `json:"quality,omitempty,omitempty"`
    // Fps int `json:"fps,omitempty"`
    // ProjectionType string `json:"projectionType,omitempty"`
    // AverageBitrate int `json:"averageBitrate,omitempty"`
    // ColorInfo ColorInfo `json:"colorInfo,omitempty"`
    // ApproxDurationMs string `json:"approxDurationMs,omitempty"`
}

type videoDetails struct {
    VideoId string `json:"videoId"`
    Title string `json:"title"`
}

type WriteCounter struct {
    Id string
    Total int64
    Total_S string
	Downloaded int64
}

func get_link_html(link string) string {
    response, err := http.Get(link)
    if err != nil {
        panic(err)
    }

    defer response.Body.Close()
    html, err := ioutil.ReadAll(response.Body)
    if err != nil {
        panic(err)
    }

    return string(html)
}

func get_main_section(html string) string {
    /*
        After some inspection, start_index and end_index is calculated
        ytplayer has some argument to it
        ending with semi-colon and ytplayer.load

        Just parsing those arguments
    */
    start_index := strings.Index(html, "\"player_response\"")
    if start_index == -1 {
        return ""
    }

    section := html[start_index : ]
    end_index := strings.Index(section, ";ytplayer.load")
    if end_index == -1 {
        return ""
    }
    section = section[ : end_index]

    return section
}

func get_json_string(html string) string {
    json_string := get_main_section(html)
    json_string = parse_out(json_string)

    start_index := strings.Index(json_string, "\"adaptiveFormats\"")
    if start_index == -1 {
        return ""
    }
    json_string = json_string[start_index + len("\"adaptiveFormats\":") : ]

    end_index := strings.Index(json_string, "]")
    if end_index == -1 {
        return ""
    }
    json_string = json_string[ : end_index + 1]

    return json_string
}

func get_jsons(json_string string) []adaptiveFormat {
    // fmt.Println(json_string)
    var adaptive_formats []adaptiveFormat
    err := json.Unmarshal([]byte(json_string), &adaptive_formats)
    if err != nil {
        // fmt.Println(err)
        panic(err)
    }

    return adaptive_formats
}

func parse_out(html string) string {
    /*
        Now for some replacing
        1. \\ with \
        2. \" with "
        3. \/ with /
        4. \u0026 with &
    */
    parsed := strings.Replace(html, "\\\\", "\\", -1)
    parsed  = strings.Replace(parsed, "\\\"", "\"", -1)
    parsed  = strings.Replace(parsed, "\\/", "/", -1)
    parsed  = strings.Replace(parsed, "\\u0026", "&", -1)

    return parsed
}

func get_video_detail(html string) videoDetails {
    title := get_main_section(html)
    title = parse_out(title)

    start_index := strings.Index(title, "\"videoDetails\"")
    if start_index == -1 {
        return videoDetails{}
    }
    title = title[start_index + len("\"videoDetails\":"):]
    // fmt.Println(title)
    end_index := strings.Index(title, "},\"playerConfig\"")
    if end_index == -1 {
        return videoDetails{}
    }
    title = title[ : end_index + 1]

    var video_detail videoDetails
    err := json.Unmarshal([]byte(title), &video_detail)
    if err != nil {
        panic(err)
    }

    return video_detail
}

func get_highest_bitrate(jsons []adaptiveFormat, codec string) int {
    highest_index := -1
    max_bitrate := 0
    for index, json := range jsons {
        if json.Bitrate > max_bitrate && strings.Contains(json.MimeType, codec) {
            max_bitrate = json.Bitrate
            highest_index = index
        }
    }

    return highest_index
}

func get_extension(json adaptiveFormat) string {
    mime := json.MimeType
    start_index := strings.Index(mime, "/")
    if start_index == -1 {
        return ""
    }
    mime = mime[start_index + 1 : ]

    end_index := strings.Index(mime, "; ")
    if end_index == -1 {
        return ""
    }
    mime = mime[ : end_index]
    mime = "." + mime

    return mime
}

func turn_to_valid_filename(text string) string {
    valid_filename := text
    valid_filename = strings.Replace(valid_filename, "?", "", -1)
    valid_filename = strings.Replace(valid_filename, ":", "", -1)
    valid_filename = strings.Replace(valid_filename, "|", "_", -1)
    valid_filename = strings.Replace(valid_filename, ".", "", -1)
    valid_filename = strings.Replace(valid_filename, "/", "", -1)
    valid_filename = strings.Replace(valid_filename, "\\", "", -1)

    return valid_filename
}

func format_integer(integer int64) string {
    tokens := []string{"B", "KiB", "MiB", "GiB", "TiB"}

    division_integer := integer
    counter := 0
    for {
        division_integer /= 1024
        if division_integer == 0 {
            break
        }
        counter += 1
    }

    float := float64(integer) / float64(math.Pow(1024, float64(counter)))
    return fmt.Sprintf("%.2f%s", float, tokens[counter])
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Downloaded += int64(n)
	wc.PrintProgress()
	return n, nil
}

func (wc WriteCounter) PrintProgress() {
	// Clear the line by using a character return to go back to the start and remove
	// the remaining characters by filling it with spaces
	fmt.Printf("\r%s", strings.Repeat(" ", 60))

	// Return again and print current status of download
	// We use the humanize package to print the bytes in a meaningful way (e.g. 10 MB)
	fmt.Printf("\r[%s] Downloading: %s/%s", wc.Id, format_integer(wc.Downloaded), wc.Total_S)
}

func get_file_size(resp *http.Response) int64 {
    size, err := strconv.Atoi(resp.Header.Get("Content-Length"))
    if err != nil {
        panic(err)
    }

    return int64(size)
}

func download_file(video_details videoDetails, json adaptiveFormat) {
    file_name := video_details.Title
    file_name = turn_to_valid_filename(file_name)
    file_name += get_extension(json)

    response, err := http.Get(json.Url)
    if err != nil {
        panic(err)
    }
    defer response.Body.Close()

    size_of_url_file := get_file_size(response)
    stat, err := os.Stat(file_name)
    if !os.IsNotExist(err) {
        if stat.Size() == size_of_url_file {
            fmt.Printf("[%s] \"%s\" exists\n", video_details.VideoId, file_name)
            return;
        }
    }

    output_file, err := os.Create(file_name)
    if err != nil {
        panic(err)
    }
    defer output_file.Close()

    counter := WriteCounter{video_details.VideoId, size_of_url_file, format_integer(size_of_url_file), 0}
    _, err = io.Copy(output_file, io.TeeReader(response.Body, &counter))
    if err != nil {
        panic(err)
    }
    fmt.Printf("\n")
}

func main() {
    if len(os.Args) == 1 {
        fmt.Printf("Youtube Audio Downloader\nUsage: %s [URL ...]\n", os.Args[0])
    }

    for index, link := range os.Args {
        if index == 0 {
            continue
        }
        if strings.Contains(link, "youtube.com") == false &&
            strings.Contains(link, "youtu.be") == false {
                fmt.Printf("\"%s\" is not a youtube link...\n", link)
                continue
        }
        html := get_link_html(link)

        video_details := get_video_detail(html)

        format_jsons := get_jsons(get_json_string(html))
        highest_bitrate_index := get_highest_bitrate(format_jsons, "audio/")
        if highest_bitrate_index == -1 {
            fmt.Printf("Link \"%s\" doesn't contains audio...\n", link)
            continue
        }

        download_file(video_details, format_jsons[highest_bitrate_index])
    }
}
