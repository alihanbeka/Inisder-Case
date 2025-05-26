package main

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

func main() {
	teams := []Team{
		{"Galatasaray", 80, 0, 0, 0, 0, 0, 0, 0},
		{"Fenerbahçe", 85, 0, 0, 0, 0, 0, 0, 0},
		{"Beşiktaş", 90, 0, 0, 0, 0, 0, 0, 0},
		{"Trabzonspor", 75, 0, 0, 0, 0, 0, 0, 0},
	}

	for _, team := range teams {
		println(team.Name, "- Güç:", team.Strength)
	}
}
