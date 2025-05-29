package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB
var league *League
var mu sync.Mutex

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "./league.db")
	if err != nil {
		log.Fatal(err)
	}

	createTeamsTable := `
    CREATE TABLE IF NOT EXISTS teams (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL,
        strength INTEGER NOT NULL
    );`

	createMatchesTable := `
    CREATE TABLE IF NOT EXISTS matches (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        home_team_id INTEGER NOT NULL,
        away_team_id INTEGER NOT NULL,
        home_goals INTEGER DEFAULT 0,
        away_goals INTEGER DEFAULT 0,
        week INTEGER NOT NULL,
        FOREIGN KEY (home_team_id) REFERENCES teams(id),
        FOREIGN KEY (away_team_id) REFERENCES teams(id)
    );`

	_, err = db.Exec(createTeamsTable)
	if err != nil {
		log.Fatal(err)
	}
	_, err = db.Exec(createMatchesTable)
	if err != nil {
		log.Fatal(err)
	}
}

func addTeam(name string, strength int) error {
	_, err := db.Exec("INSERT INTO teams (name, strength) VALUES (?, ?)", name, strength)
	return err
}

func addMatch(homeID, awayID, homeGoals, awayGoals, week int) error {
	_, err := db.Exec("INSERT INTO matches (home_team_id, away_team_id, home_goals, away_goals, week) VALUES (?, ?, ?, ?, ?)",
		homeID, awayID, homeGoals, awayGoals, week)
	return err
}

func getTeams() ([]Team, error) {
	rows, err := db.Query("SELECT id, name, strength FROM teams")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []Team
	for rows.Next() {
		var t Team
		if err := rows.Scan(&t.ID, &t.Name, &t.Strength); err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, nil
}

func tableHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(league.Teams)
}

func playAllHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	if league.Week >= league.TotalWeeks {
		http.Error(w, "Ligde oynanacak hafta kalmadı", http.StatusBadRequest)
		return
	}
	league.PlayAllWeeks()
	resp := map[string]string{"message": "Tüm lig maçları oynandı"}
	json.NewEncoder(w).Encode(resp)
}

func resetHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	league.Week = 0
	for _, team := range league.Teams {
		t := team.(*Team)
		*t = Team{ID: t.ID, Name: t.Name, Strength: t.Strength}
	}

	resp := map[string]string{"message": "Lig sıfırlandı"}
	json.NewEncoder(w).Encode(resp)
}

func playWeekHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	if league.Week >= league.TotalWeeks {
		http.Error(w, "Ligde oynanacak hafta kalmadı", http.StatusBadRequest)
		return
	}

	league.PlayWeek()
	resp := map[string]interface{}{"week": league.Week, "message": "Hafta oynandı"}
	json.NewEncoder(w).Encode(resp)
}

func matchesByWeekHandler(w http.ResponseWriter, r *http.Request) {
	weekStr := strings.TrimPrefix(r.URL.Path, "/matches/week/")
	week, err := strconv.Atoi(weekStr)
	if err != nil {
		http.Error(w, "Geçersiz hafta", http.StatusBadRequest)
		return
	}

	query := `
	SELECT m.id, t1.name, t2.name, m.home_goals, m.away_goals 
	FROM matches m
	JOIN teams t1 ON m.home_team_id = t1.id
	JOIN teams t2 ON m.away_team_id = t2.id
	WHERE m.week = ?
	`
	rows, err := db.Query(query, week)
	if err != nil {
		http.Error(w, "Veritabanı hatası", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var matches []map[string]interface{}
	for rows.Next() {
		var id, homeGoals, awayGoals int
		var home, away string
		rows.Scan(&id, &home, &away, &homeGoals, &awayGoals)
		matches = append(matches, map[string]interface{}{
			"id":        id,
			"homeTeam":  home,
			"awayTeam":  away,
			"homeGoals": homeGoals,
			"awayGoals": awayGoals,
		})
	}
	json.NewEncoder(w).Encode(matches)
}

func updateMatchHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Geçersiz ID", http.StatusBadRequest)
		return
	}
	var input struct {
		HomeGoals int `json:"homeGoals"`
		AwayGoals int `json:"awayGoals"`
	}
	json.NewDecoder(r.Body).Decode(&input)

	var match *Match
	for _, m := range league.Matches {
		if mm := m.(*Match); mm.ID == id {
			match = mm
			break
		}
	}
	if match == nil {
		http.Error(w, "Maç bulunamadı", http.StatusNotFound)
		return
	}

	removeMatchStats(match)
	match.HomeGoals = input.HomeGoals
	match.AwayGoals = input.AwayGoals
	match.Home.UpdateStats(input.HomeGoals, input.AwayGoals, true)
	match.Away.UpdateStats(input.HomeGoals, input.AwayGoals, false)

	json.NewEncoder(w).Encode(map[string]string{"message": "Maç sonucu güncellendi"})
}

func safeDecrement(value *int, amount int) {
	*value -= amount
	if *value < 0 {
		*value = 0
	}
}

func removeMatchStats(m *Match) {
	home := m.Home.(*Team)
	away := m.Away.(*Team)

	safeDecrement(&home.Played, 1)
	safeDecrement(&away.Played, 1)

	safeDecrement(&home.GoalsFor, m.HomeGoals)
	safeDecrement(&home.GoalsAgainst, m.AwayGoals)

	safeDecrement(&away.GoalsFor, m.AwayGoals)
	safeDecrement(&away.GoalsAgainst, m.HomeGoals)

	if m.HomeGoals > m.AwayGoals {
		safeDecrement(&home.Won, 1)
		safeDecrement(&home.Points, 3)
		safeDecrement(&away.Lost, 1)
	} else if m.HomeGoals < m.AwayGoals {
		safeDecrement(&away.Won, 1)
		safeDecrement(&away.Points, 3)
		safeDecrement(&home.Lost, 1)
	} else {
		safeDecrement(&home.Drawn, 1)
		safeDecrement(&away.Drawn, 1)
		safeDecrement(&home.Points, 1)
		safeDecrement(&away.Points, 1)
	}
}

// --- Structlar ---

type TeamInterface interface {
	GetName() string
	GetStrength() int
	UpdateStats(homeGoals, awayGoals int, isHome bool)
	GetStats() string
}

type MatchInterface interface {
	Play()
	GetResult() (int, int)
}

type Team struct {
	ID           int
	Name         string
	Strength     int
	Played       int
	Won          int
	Drawn        int
	Lost         int
	GoalsFor     int
	GoalsAgainst int
	Points       int
}

func (t *Team) GetName() string  { return t.Name }
func (t *Team) GetStrength() int { return t.Strength }
func (t *Team) GetStats() string { return fmt.Sprintf("%s - %d P", t.Name, t.Points) }
func (t *Team) UpdateStats(hg, ag int, isHome bool) {
	t.Played++
	if isHome {
		t.GoalsFor += hg
		t.GoalsAgainst += ag
	} else {
		t.GoalsFor += ag
		t.GoalsAgainst += hg
	}
	if hg > ag {
		if isHome {
			t.Won++
			t.Points += 3
		} else {
			t.Lost++
		}
	} else if hg < ag {
		if isHome {
			t.Lost++
		} else {
			t.Won++
			t.Points += 3
		}
	} else {
		t.Drawn++
		t.Points++
	}
}

type Match struct {
	ID        int
	Home      TeamInterface
	Away      TeamInterface
	HomeGoals int
	AwayGoals int
}

func (m *Match) Play() {
	rand.Seed(time.Now().UnixNano())
	maxHomeGoals := m.Home.GetStrength() / 10
	maxAwayGoals := m.Away.GetStrength() / 10
	if maxHomeGoals > 5 {
		maxHomeGoals = 5
	}
	if maxAwayGoals > 5 {
		maxAwayGoals = 5
	}
	m.HomeGoals = rand.Intn(maxHomeGoals + 1)
	m.AwayGoals = rand.Intn(maxAwayGoals + 1)
	m.Home.UpdateStats(m.HomeGoals, m.AwayGoals, true)
	m.Away.UpdateStats(m.HomeGoals, m.AwayGoals, false)
}

func (m *Match) GetResult() (int, int) {
	return m.HomeGoals, m.AwayGoals
}

type League struct {
	Teams      []TeamInterface
	Matches    []MatchInterface
	Week       int
	TotalWeeks int
}

func (l *League) PlayWeek() {
	start := l.Week * 2
	end := start + 2
	if end > len(l.Matches) {
		end = len(l.Matches)
	}
	for i := start; i < end; i++ {
		l.Matches[i].Play()
		hg, ag := l.Matches[i].GetResult()
		fmt.Printf("Hafta %d: %s %d - %d %s\n", l.Week+1,
			l.Matches[i].(*Match).Home.GetName(), hg, ag, l.Matches[i].(*Match).Away.GetName())
	}
	l.Week++
}

func (l *League) PlayAllWeeks() {
	for l.Week < l.TotalWeeks {
		l.PlayWeek()
	}
}

func (l *League) GetTable() []TeamInterface {
	return l.Teams
}

func generateFixtures(teams []TeamInterface) []Match {
	var fixtures []Match
	n := len(teams)
	if n%2 != 0 {
		return fixtures
	}
	totalWeeks := n - 1
	matchesPerWeek := n / 2

	for week := 0; week < totalWeeks; week++ {
		for i := 0; i < matchesPerWeek; i++ {
			home := teams[i]
			away := teams[n-1-i]
			fixtures = append(fixtures, Match{Home: home, Away: away})
		}
		teams = rotateTeamInterfaces(teams)
	}
	return fixtures
}

func rotateTeamInterfaces(teams []TeamInterface) []TeamInterface {
	n := len(teams)
	newOrder := make([]TeamInterface, n)
	newOrder[0] = teams[0]
	for i := 1; i < n-1; i++ {
		newOrder[i] = teams[i+1]
	}
	newOrder[n-1] = teams[1]
	return newOrder
}

func main() {
	initDB()

	// Takımları veritabanına sadece bir kere ekle
	teams, err := getTeams()
	if err != nil {
		log.Fatal("Takımlar alınamadı:", err)
	}
	if len(teams) == 0 {
		addTeam("Galatasaray", 90)
		addTeam("Fenerbahçe", 85)
		addTeam("Beşiktaş", 80)
		addTeam("Trabzonspor", 75)

		teams, err = getTeams()
		if err != nil {
			log.Fatal("Takımlar alınamadı:", err)
		}
	}

	// Takımları interface listesine dönüştür
	var teamInterfaces []TeamInterface
	var teamStructs []Team
	for i := range teams {
		team := teams[i]
		teamCopy := team // pointer hatası yaşamamak için kopya
		teamInterfaces = append(teamInterfaces, &teamCopy)
		teamStructs = append(teamStructs, team)
	}

	// Fikstür oluştur
	rawMatches := generateFixtures(teamInterfaces)

	var matches []MatchInterface
	week := 1
	for _, m := range rawMatches {
		home := m.Home.(*Team)
		away := m.Away.(*Team)
		addMatch(home.ID, away.ID, 0, 0, week)
		matches = append(matches, &Match{
			Home: home,
			Away: away,
		})
		week++
	}

	// Ligi başlat
	league = &League{
		Teams:      teamInterfaces,
		Matches:    matches,
		Week:       0,
		TotalWeeks: len(matches) / 2,
	}

	// API route'ları
	http.HandleFunc("/reset", resetHandler)
	http.HandleFunc("/playweek", playWeekHandler)
	http.HandleFunc("/table", tableHandler)
	http.HandleFunc("/playall", playAllHandler)
	http.HandleFunc("/match", updateMatchHandler)
	http.HandleFunc("/matches/week/", matchesByWeekHandler)

	fmt.Println("Server 8080 portunda başladı...")
	http.ListenAndServe(":8080", nil)
}
