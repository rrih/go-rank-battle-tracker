package Handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// シーズンごとのデータを表す構造体
type SeasonData struct {
	CID     string  `json:"cId"`
	Cnt     float64 `json:"cnt"`
	End     string  `json:"end"`
	Name    string  `json:"name"`
	RankCnt int     `json:"rankCnt"`
	Rst     int     `json:"rst"`
	Rule    int     `json:"rule"`
	Season  int     `json:"season"`
	Start   string  `json:"start"`
	Ts1     float64 `json:"ts1"`
	Ts2     float64 `json:"ts2"`
}

// シーズンリスト
type SeasonList struct {
	Seasons map[string]map[string]SeasonData `json:"list"`
}

// ランキング
type RankResponseRawData struct {
	Rank        int     `json:"rank"`
	RatingValue float64 `json:"rating_value"`
	Icon        string  `json:"icon"`
	Name        string  `json:"name"`
	Lng         string  `json:"lng"`
}

// レスポンス
type RankingResponse struct {
	SeasonData SeasonData            `json:"season_data"`
	Top1000    []RankResponseRawData `json:"top_1000"`
}

func fetchRankingData() (*SeasonList, error) {
	req, err := http.NewRequest("POST", "https://api.battle.pokemon-home.com/tt/cbd/competition/rankmatch/list", strings.NewReader(`{"soft": "Sc"}`))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "ja,en-US;q=0.9,en;q=0.8")
	req.Header.Set("Origin", "https://resource.pokemon-home.com")
	req.Header.Set("Referer", "https://resource.pokemon-home.com/")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-site")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch data, status code: %d", resp.StatusCode)
	}

	var seasonList SeasonList
	err = json.NewDecoder(resp.Body).Decode(&seasonList)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	for _, season := range seasonList.Seasons {
		for _, seasonData := range season {
			startDate := strings.Replace(seasonData.Start, "/", "-", -1) + ":00"
			start, err := time.Parse("2006-01-02 15:04:05", startDate)
			if err != nil {
				return nil, fmt.Errorf("failed to parse start time: %v", err)
			}
			endDate := strings.Replace(seasonData.End, "/", "-", -1) + ":00"
			end, err := time.Parse("2006-01-02 15:04:05", endDate)
			if err != nil {
				return nil, fmt.Errorf("failed to parse end time: %v", err)
			}
			seasonData.Start = start.Format("2006-01-02 15:04:05")
			seasonData.End = end.Format("2006-01-02 15:04:05")
		}
	}

	return &seasonList, nil
}

// 最新の1000位までのランキングデータを取得
func fetchTop1000RankingData(cId string, rst int, ts1 string) ([]RankResponseRawData, error) {
	rankingURL := fmt.Sprintf("https://resource.pokemon-home.com/battledata/ranking/scvi/%s/%d/%s/traner-1", cId, rst, ts1)
	resp, err := http.Get(rankingURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch top 1000 ranking data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch top 1000 ranking data, status code: %d", resp.StatusCode)
	}

	var rankingData []RankResponseRawData
	err = json.NewDecoder(resp.Body).Decode(&rankingData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ranking data: %v", err)
	}

	if len(rankingData) < 1000 {
		return nil, fmt.Errorf("top 1000 ranking data is less than 1000")
	}

	rankingResponse := convertRawDataToResponse(rankingData)
	return rankingResponse, nil
}

// ランキングの元データから変換
func convertRawDataToResponse(rawData []RankResponseRawData) []RankResponseRawData {
	result := make([]RankResponseRawData, len(rawData))
	for i, data := range rawData {
		iconURL := fmt.Sprintf("https://resource.pokemon-home.com/battledata/img/icons/trainer/%s", data.Icon)
		result[i].Icon = iconURL
		result[i].RatingValue = data.RatingValue / 1000
		result[i].Rank = data.Rank
		result[i].Name = data.Name
		result[i].Lng = data.Lng
	}
	return result
}

// endpoint handler
func RankingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	seasonList, err := fetchRankingData()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching ranking data: %v", err), http.StatusInternalServerError)
		return
	}

	// 最新のシーズンデータ取得
	latestSeasonData, err := getLatestSeasonData(seasonList.Seasons)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching latest season data: %v", err), http.StatusInternalServerError)
		return
	}

	// 上位1000位のランキングデータ取得
	top1000Data, err := fetchTop1000RankingData(latestSeasonData.CID, latestSeasonData.Rst, fmt.Sprintf("%.0f", latestSeasonData.Ts1))
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching top 1000 ranking data: %v", err), http.StatusInternalServerError)
		return
	}

	responseData := RankingResponse{
		SeasonData: latestSeasonData,
		Top1000:    top1000Data,
	}

	if err := json.NewEncoder(w).Encode(responseData); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}
}

func Handler() {
	http.HandleFunc("/rankings", RankingHandler)

	fmt.Println("Server is running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// 最新のシーズンデータ取得
func getLatestSeasonData(seasons map[string]map[string]SeasonData) (SeasonData, error) {
	now := time.Now()
	// 現在時刻がシーズンの開始日時と終了日時の間にあるものを取得
	for _, season := range seasons {
		for _, seasonData := range season {
			seasonData.Start = strings.Replace(seasonData.Start, "/", "-", -1) + ":00"
			start, err := time.Parse("2006-01-02 15:04:05", seasonData.Start)
			if err != nil {
				return SeasonData{}, fmt.Errorf("failed to parse start time: %v", err)
			}
			seasonData.End = strings.Replace(seasonData.End, "/", "-", -1) + ":00"
			end, err := time.Parse("2006-01-02 15:04:05", seasonData.End)
			if err != nil {
				return SeasonData{}, fmt.Errorf("failed to parse end time: %v", err)
			}
			if now.After(start) && now.Before(end) {
				return seasonData, nil
			}
		}
	}
	return SeasonData{}, fmt.Errorf("no season data available")
}
