package collector

import (
	"io"
	"strings"
	"wdata"

	"golang.org/x/net/html"
)

const (
	htmlTagTableBody = "tbody"
	htmlTagTable     = "table"
	htmlTagTr        = "tr"
	htmlTagTh        = "th"
	htmlTagTd        = "td"
	htmlTagImg       = "img"
)

func textWanted(text string) bool {
	return !strings.Contains(text, "白天") && !strings.Contains(text, "晚上") && len([]rune(text)) > 0
}

func parseDate(tokenizer *html.Tokenizer) wdata.Dates {
	var dateArr wdata.Dates
	var date string
	for {
		tokenType, token := tokenizer.Next(), tokenizer.Token()
		if tokenType == html.EndTagToken && token.Data == htmlTagTr {
			break
		}

		if tokenType == html.EndTagToken && token.Data == htmlTagTh {
			if len(date) > 0 {
				dateArr = append(dateArr, strings.TrimPrefix(date, ":"))
				date = ""
			}
			continue
		}

		if tokenType == html.TextToken {
			text := strings.Trim(token.Data, " \n\t")
			if textWanted(text) {
				date = strings.Join([]string{date, text}, ":")
			}
		}
	}

	return dateArr
}

func parseCity(tokenizer *html.Tokenizer) string {
	tokenType, token := tokenizer.Next(), tokenizer.Token()
	var city string
	if tokenType == html.TextToken {
		city = strings.Trim(token.Data, " \n\t")
	}
	return city
}

func parseWeatherData(tokenizer *html.Tokenizer) (wdata.Temperatures, wdata.Temperatures) {

	parseTemperature := func() wdata.Temperatures {
		var tempArr wdata.Temperatures
		var text string
		for {
			tokenType, token := tokenizer.Next(), tokenizer.Token()
			// Break when parsing to </tr>
			if tokenType == html.EndTagToken && token.Data == htmlTagTr {
				break
			}

			if tokenType == html.EndTagToken && token.Data == htmlTagTd {
				if len(text) > 0 {
					tempArr = append(tempArr, strings.TrimPrefix(text, ":"))
					text = ""
				}
				continue
			}

			if tokenType == html.SelfClosingTagToken && token.Data == htmlTagImg {
				text = strings.Join([]string{text, strings.Trim(token.Attr[2].Val, " \n\t")}, ":")
				continue
			}

			if tokenType == html.TextToken {
				data := strings.Trim(token.Data, " \n\t")
				if textWanted(data) {
					text = strings.Join([]string{text, strings.Trim(data, " \n\t")}, ":")
				}
			}
		}
		return tempArr
	}

	var dayWeather, nightWeather wdata.Temperatures
	for {
		if len(dayWeather) == 7 && len(nightWeather) == 7 {
			break
		}

		tokenType, token := tokenizer.Next(), tokenizer.Token()

		if tokenType == html.StartTagToken && token.Data == htmlTagTd {
			dayWeather = parseTemperature()
		}

		if tokenType == html.StartTagToken && token.Data == htmlTagTd {
			nightWeather = parseTemperature()
		}
	}

	return dayWeather, nightWeather
}

func parseWeeklyData(tokenizer *html.Tokenizer) *wdata.WeatherInfoCollection {
	traversToTHTag := func() html.Token {
		var token html.Token
		tokenType := html.ErrorToken
		for token.Data != htmlTagTh && tokenType != html.StartTagToken {
			tokenType, token = tokenizer.Next(), tokenizer.Token()
		}
		return token
	}

	var collection = new(wdata.WeatherInfoCollection)
	collection.Weathers = map[string]*wdata.WeeklyWeatherInfo{}
	for {
		tokenType, token := tokenizer.Next(), tokenizer.Token()
		if tokenType == html.EndTagToken && token.Data == htmlTagTableBody {
			break
		}

		if tokenType == html.StartTagToken && token.Data == htmlTagTr {
			// Check first child <th> tag for checking what the row stands for.
			token = traversToTHTag()

			if len(token.Attr) == 1 {
				if collection.HasDate() == false {
					collection.SetDate(parseDate(tokenizer))
				}
			} else {
				var info = new(wdata.WeeklyWeatherInfo)
				info.City = parseCity(tokenizer)
				dayWeatherData, nightWeatherData := parseWeatherData(tokenizer)
				info.DayData = dayWeatherData
				info.NightData = nightWeatherData
				collection.Weathers[info.City] = info
			}
		}
	}
	return collection
}

func parseHTML(r io.Reader) *wdata.WeatherInfoCollection {
	tokenizer := html.NewTokenizer(r)
	for {
		tokenType, token := tokenizer.Next(), tokenizer.Token()
		if tokenType == html.StartTagToken && token.Data == htmlTagTable {
			return parseWeeklyData(tokenizer)
		}

		if tokenType == html.ErrorToken {
			// Handle end of file
			err := tokenizer.Err()
			if err == io.EOF {
				break
			}
		}
	}
	return nil
}