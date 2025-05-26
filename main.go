package main

import (
	"fmt"
	"math/rand"
	"time"
)

// --- Interface tanımları ---

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

type LeagueInterface interface {
	PlayWeek()
	GetTable() []TeamInterface
}

// --- Struct ve metodlar ---

type Team struct {
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

func (t *Team) GetName() string {
	return t.Name
}

func (t *Team) GetStrength() int {
	return t.Strength
}

func (t *Team) UpdateStats(homeGoals, awayGoals int, isHome bool) {
	t.Played++
	if isHome {
		t.GoalsFor += homeGoals
		t.GoalsAgainst += awayGoals
	} else {
		t.GoalsFor += awayGoals
		t.GoalsAgainst += homeGoals
	}

	if homeGoals > awayGoals {
		if isHome {
			t.Won++
			t.Points += 3
		} else {
			t.Lost++
		}
	} else if homeGoals < awayGoals {
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

func (t *Team) GetStats() string {
	return fmt.Sprintf("%s - Puan: %d, Oynanan: %d, G: %d, M: %d, B: %d, AG: %d, YG: %d",
		t.Name, t.Points, t.Played, t.Won, t.Lost, t.Drawn, t.GoalsFor, t.GoalsAgainst)
}

type Match struct {
	Home      TeamInterface
	Away      TeamInterface
	HomeGoals int
	AwayGoals int
}

func (m *Match) Play() {
	rand.Seed(time.Now().UnixNano())

	maxHomeGoals := m.Home.GetStrength() / 10
	if maxHomeGoals > 5 {
		maxHomeGoals = 5
	}
	maxAwayGoals := m.Away.GetStrength() / 10
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
	Teams   []TeamInterface
	Matches []MatchInterface
	Week    int
}

func (l *League) PlayWeek() {
	start := l.Week * 2
	end := start + 2
	if end > len(l.Matches) {
		end = len(l.Matches)
	}
	for i := start; i < end; i++ {
		l.Matches[i].Play()
		homeGoals, awayGoals := l.Matches[i].GetResult()
		fmt.Printf("Hafta %d: %s %d - %d %s\n", l.Week+1,
			l.Matches[i].(*Match).Home.GetName(),
			homeGoals,
			awayGoals,
			l.Matches[i].(*Match).Away.GetName())
	}
	l.Week++
}

func (l *League) GetTable() []TeamInterface {
	return l.Teams
}

// --- main fonksiyonu ---

func main() {
	teams := []TeamInterface{
		&Team{Name: "Galatasaray", Strength: 90},
		&Team{Name: "Fenerbahçe", Strength: 85},
		&Team{Name: "Beşiktaş", Strength: 80},
		&Team{Name: "Trabzonspor", Strength: 75},
	}

	matches := []MatchInterface{
		&Match{Home: teams[0], Away: teams[1]},
		&Match{Home: teams[2], Away: teams[3]},
		&Match{Home: teams[0], Away: teams[2]},
		&Match{Home: teams[1], Away: teams[3]},
		&Match{Home: teams[0], Away: teams[3]},
		&Match{Home: teams[1], Away: teams[2]},
	}

	league := &League{
		Teams:   teams,
		Matches: matches,
		Week:    0,
	}

	for i := 0; i < 3; i++ {
		league.PlayWeek()
		fmt.Println()
	}

	fmt.Println("Lig Tablosu:")
	for _, team := range league.GetTable() {
		fmt.Println(team.GetStats())
	}
}
