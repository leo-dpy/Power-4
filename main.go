package main

import (
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

type Gravity int

const (
	GravityDown Gravity = iota
	GravityUp
)

type GameMode int

const (
	ModeHumanVsHuman GameMode = iota
	ModeHumanVsAI
)

type AILevel int

const (
	AIEasy AILevel = iota
	AIMedium
	AIHard
)

// Ajoute un champ Mode à Game pour retenir le mode de jeu
type Game struct {
	Board         [][]int
	Rows, Cols    int
	CurrentPlayer int
	Winner        int
	GameOver      bool
	LastRow       int
	LastCol       int
	TurnCount     int
	Gravity       Gravity
	Difficulty    string
	Username      string // kept for backward compatibility
	Username1     string
	Username2     string
	Mode          string // "normal" ou "inverse"
	GameMode      GameMode
	AILevel       AILevel
	Skin          string // Nom du skin sélectionné
}

var (
	game  *Game
	mutex sync.Mutex
)

// Durée du délai en millisecondes entre le coup du joueur et celui de l'IA
var aiDelayMs = 1000

func NewGame(rows, cols, prefill int, difficulty, username1, username2, mode, skin string, gameMode GameMode, aiLevel AILevel) *Game {
	board := make([][]int, rows)
	for i := range board {
		board[i] = make([]int, cols)
	}
	// Prefill random cells
	source := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(source)
	for n := 0; n < prefill; {
		r := rng.Intn(rows)
		c := rng.Intn(cols)
		if board[r][c] == 0 {
			board[r][c] = rng.Intn(2) + 1
			n++
		}
	}
	gravity := GravityDown
	if mode == "inverse" {
		gravity = GravityUp
	} else {
		gravity = GravityDown
	}
	if gameMode == ModeHumanVsAI && username2 == "" {
		username2 = "IA"
	}
	return &Game{
		Board:         board,
		Rows:          rows,
		Cols:          cols,
		CurrentPlayer: 1,
		Winner:        0,
		GameOver:      false,
		LastRow:       -1,
		LastCol:       -1,
		TurnCount:     0,
		Gravity:       gravity,
		Difficulty:    difficulty,
		Username:      username1,
		Username1:     username1,
		Username2:     username2,
		Mode:          mode,
		GameMode:      gameMode,
		AILevel:       aiLevel,
		Skin:          skin,
	}
}

// DropToken now supports gravity direction and increments turn count.
func (g *Game) DropToken(col int) bool {
	if col < 0 || col >= g.Cols || g.GameOver {
		return false
	}
	var row int
	if g.Gravity == GravityDown {
		for row = g.Rows - 1; row >= 0; row-- {
			if g.Board[row][col] == 0 {
				break
			}
		}
	} else {
		for row = 0; row < g.Rows; row++ {
			if g.Board[row][col] == 0 {
				break
			}
		}
	}
	if row < 0 || row >= g.Rows || g.Board[row][col] != 0 {
		return false
	}
	g.Board[row][col] = g.CurrentPlayer
	g.LastRow = row
	g.LastCol = col
	g.TurnCount++
	// Gravity reversal every 5 turns - only in inverse mode
	if g.Mode == "inverse" && g.TurnCount%5 == 0 {
		if g.Gravity == GravityDown {
			g.Gravity = GravityUp
		} else {
			g.Gravity = GravityDown
		}
	}
	if g.checkWin(row, col) {
		g.Winner = g.CurrentPlayer
		g.GameOver = true
	} else if g.isDraw() {
		g.GameOver = true
	}
	g.CurrentPlayer = 3 - g.CurrentPlayer
	return true
}

// checkWin vérifie si le dernier coup joué (row, col) crée un alignement de 4 jetons de même couleur.
func (g *Game) checkWin(row, col int) bool {
	player := g.Board[row][col]
	dirs := [][2]int{{0, 1}, {1, 0}, {1, 1}, {1, -1}}
	for _, d := range dirs {
		count := 1
		for i := 1; i < 4; i++ {
			r := row + d[0]*i
			c := col + d[1]*i
			if r >= 0 && r < g.Rows && c >= 0 && c < g.Cols && g.Board[r][c] == player {
				count++
			} else {
				break
			}
		}
		for i := 1; i < 4; i++ {
			r := row - d[0]*i
			c := col - d[1]*i
			if r >= 0 && r < g.Rows && c >= 0 && c < g.Cols && g.Board[r][c] == player {
				count++
			} else {
				break
			}
		}
		if count >= 4 {
			return true
		}
	}
	return false
}

// isDraw vérifie si le plateau est plein (aucune case vide en haut de chaque colonne).
func (g *Game) isDraw() bool {
	for c := 0; c < g.Cols; c++ {
		if g.Board[0][c] == 0 {
			return false
		}
	}
	return true
}

// AI Functions

// getValidMoves retourne les colonnes où il est possible de jouer
func (g *Game) getValidMoves() []int {
	var moves []int
	for col := 0; col < g.Cols; col++ {
		// Vérifie si la colonne n'est pas pleine
		var canPlay bool
		if g.Gravity == GravityDown {
			canPlay = g.Board[0][col] == 0
		} else {
			canPlay = g.Board[g.Rows-1][col] == 0
		}
		if canPlay {
			moves = append(moves, col)
		}
	}
	return moves
}

// checkWinningMove vérifie si jouer dans une colonne ferait gagner le joueur
func (g *Game) checkWinningMove(col, player int) bool {
	// Simule le coup
	var row int
	if g.Gravity == GravityDown {
		for row = g.Rows - 1; row >= 0; row-- {
			if g.Board[row][col] == 0 {
				break
			}
		}
	} else {
		for row = 0; row < g.Rows; row++ {
			if g.Board[row][col] == 0 {
				break
			}
		}
	}

	if row < 0 || row >= g.Rows || g.Board[row][col] != 0 {
		return false
	}

	// Place temporairement le jeton
	g.Board[row][col] = player
	win := g.checkWin(row, col)
	g.Board[row][col] = 0 // Retire le jeton

	return win
}

// aiEasyMove - IA facile : joue aléatoirement
func (g *Game) aiEasyMove() int {
	moves := g.getValidMoves()
	if len(moves) == 0 {
		return -1
	}
	return moves[rand.Intn(len(moves))]
}

// aiMediumMove - IA moyenne : bloque les victoires adverses et cherche ses victoires
func (g *Game) aiMediumMove() int {
	moves := g.getValidMoves()
	if len(moves) == 0 {
		return -1
	}

	// 1. Cherche un coup gagnant pour l'IA (joueur 2)
	for _, col := range moves {
		if g.checkWinningMove(col, 2) {
			return col
		}
	}

	// 2. Bloque un coup gagnant de l'adversaire (joueur 1)
	for _, col := range moves {
		if g.checkWinningMove(col, 1) {
			return col
		}
	}

	// 3. Sinon, joue aléatoirement
	return moves[rand.Intn(len(moves))]
}

// aiHardMove - IA difficile : utilise minimax
func (g *Game) aiHardMove() int {
	moves := g.getValidMoves()
	if len(moves) == 0 {
		return -1
	}

	// Utilise minimax avec une profondeur limitée
	_, bestCol := g.minimax(4, true, -1000, 1000)

	// Fallback au cas où minimax échoue
	if bestCol == -1 && len(moves) > 0 {
		return moves[0]
	}

	return bestCol
}

// minimax - Algorithme minimax avec élagage alpha-beta
func (g *Game) minimax(depth int, isMaximizing bool, alpha, beta int) (int, int) {
	// Conditions de fin
	if depth == 0 || g.GameOver {
		return g.evaluateBoard(), -1
	}

	moves := g.getValidMoves()
	if len(moves) == 0 {
		return 0, -1 // Match nul
	}

	bestCol := moves[0]

	if isMaximizing {
		maxEval := -1000
		for _, col := range moves {
			// Simule le coup
			row := g.simulateMove(col, 2)
			if row == -1 {
				continue
			}

			eval, _ := g.minimax(depth-1, false, alpha, beta)
			g.Board[row][col] = 0 // Annule le coup

			if eval > maxEval {
				maxEval = eval
				bestCol = col
			}

			alpha = max(alpha, eval)
			if beta <= alpha {
				break // Élagage alpha-beta
			}
		}
		return maxEval, bestCol
	} else {
		minEval := 1000
		for _, col := range moves {
			// Simule le coup
			row := g.simulateMove(col, 1)
			if row == -1 {
				continue
			}

			eval, _ := g.minimax(depth-1, true, alpha, beta)
			g.Board[row][col] = 0 // Annule le coup

			if eval < minEval {
				minEval = eval
				bestCol = col
			}

			beta = min(beta, eval)
			if beta <= alpha {
				break // Élagage alpha-beta
			}
		}
		return minEval, bestCol
	}
}

// simulateMove simule un coup sans vérifier les conditions de victoire
func (g *Game) simulateMove(col, player int) int {
	var row int
	if g.Gravity == GravityDown {
		for row = g.Rows - 1; row >= 0; row-- {
			if g.Board[row][col] == 0 {
				break
			}
		}
	} else {
		for row = 0; row < g.Rows; row++ {
			if g.Board[row][col] == 0 {
				break
			}
		}
	}

	if row < 0 || row >= g.Rows || g.Board[row][col] != 0 {
		return -1
	}

	g.Board[row][col] = player
	return row
}

// evaluateBoard évalue la position pour l'IA (joueur 2)
func (g *Game) evaluateBoard() int {
	score := 0

	// Vérifie toutes les fenêtres de 4 cases
	for r := 0; r < g.Rows; r++ {
		for c := 0; c < g.Cols; c++ {
			// Horizontal
			if c+3 < g.Cols {
				score += g.evaluateWindow(r, c, 0, 1)
			}
			// Vertical
			if r+3 < g.Rows {
				score += g.evaluateWindow(r, c, 1, 0)
			}
			// Diagonale descendante
			if r+3 < g.Rows && c+3 < g.Cols {
				score += g.evaluateWindow(r, c, 1, 1)
			}
			// Diagonale montante
			if r+3 < g.Rows && c-3 >= 0 {
				score += g.evaluateWindow(r, c, 1, -1)
			}
		}
	}

	return score
}

// evaluateWindow évalue une fenêtre de 4 cases
func (g *Game) evaluateWindow(startR, startC, deltaR, deltaC int) int {
	score := 0
	aiCount := 0
	humanCount := 0

	for i := 0; i < 4; i++ {
		r := startR + i*deltaR
		c := startC + i*deltaC

		if g.Board[r][c] == 2 {
			aiCount++
		} else if g.Board[r][c] == 1 {
			humanCount++
		}
	}

	// Ne peut pas être une ligne gagnante si les deux joueurs y ont des jetons
	if aiCount > 0 && humanCount > 0 {
		return 0
	}

	// Évaluation pour l'IA (joueur 2)
	if aiCount == 4 {
		score += 100
	} else if aiCount == 3 {
		score += 10
	} else if aiCount == 2 {
		score += 2
	}

	// Évaluation contre l'humain (joueur 1)
	if humanCount == 4 {
		score -= 100
	} else if humanCount == 3 {
		score -= 10
	} else if humanCount == 2 {
		score -= 2
	}

	return score
}

// Fonctions utilitaires pour min/max
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// aiMove choisit le coup de l'IA selon son niveau
func (g *Game) aiMove() int {
	switch g.AILevel {
	case AIEasy:
		return g.aiEasyMove()
	case AIMedium:
		return g.aiMediumMove()
	case AIHard:
		return g.aiHardMove()
	default:
		return g.aiEasyMove()
	}
}

// getWinningPositions retourne les positions des 4 jetons gagnants si victoire, sinon nil.
func (g *Game) getWinningPositions() [][2]int {
	player := g.Winner
	if player == 0 {
		return nil
	}
	dirs := [][2]int{{0, 1}, {1, 0}, {1, 1}, {1, -1}}
	for r := 0; r < g.Rows; r++ {
		for c := 0; c < g.Cols; c++ {
			if g.Board[r][c] != player {
				continue
			}
			for _, d := range dirs {
				positions := [][2]int{{r, c}}
				for i := 1; i < 4; i++ {
					r2 := r + d[0]*i
					c2 := c + d[1]*i
					if r2 >= 0 && r2 < g.Rows && c2 >= 0 && c2 < g.Cols && g.Board[r2][c2] == player {
						positions = append(positions, [2]int{r2, c2})
					} else {
						break
					}
				}
				if len(positions) == 4 {
					return positions
				}
			}
		}
	}
	return nil
}

// renderBoard génère le HTML du plateau et permet la sélection de colonne par clic sur la flèche au-dessus de chaque colonne.
// Les boutons de colonne ont été remplacés par cette interaction directe, plus intuitive.
func renderBoard(g *Game) template.HTML {
	playerClass := "p1"
	if g.CurrentPlayer == 2 {
		playerClass = "p2"
	}

	// Désactive l'interface si c'est le tour de l'IA
	disableInterface := g.GameMode == ModeHumanVsAI && g.CurrentPlayer == 2 && !g.GameOver
	// Plus de flèches directionnelles: clic direct sur la colonne
	winning := map[[2]int]bool{}
	if g.GameOver && g.Winner != 0 {
		for _, pos := range g.getWinningPositions() {
			winning[pos] = true
		}
	}
	html := "<form method='POST' id='board-form'><input type='hidden' name='col' id='col-input'/>\n"
	html += "<div class='board-wrap " + playerClass
	if g.Gravity == GravityUp {
		html += " gravity-up"
	} else {
		html += " gravity-down"
	}
	html += "' id='board-wrap' style='overflow-x:auto; max-width:100vw;'>\n"
	html += "<table class='board' id='board' data-gameover='"
	if g.GameOver {
		html += "1'"
	} else {
		html += "0'"
	}
	html += " data-current='" + strconv.Itoa(g.CurrentPlayer) + "' style='margin:auto;'>\n"

	// Suppression de la ligne de sélection: on clique désormais directement sur une colonne du plateau

	// Plateau de jeu
	for r := 0; r < g.Rows; r++ {
		html += "<tr>"
		for c := 0; c < g.Cols; c++ {
			cell := ""
			tokenCls := ""
			wrapCls := ""
			if winning[[2]int{r, c}] {
				tokenCls = " winner-token"
			}
			if g.LastRow == r && g.LastCol == c {
				wrapCls = " just-played"
			}
			switch g.Board[r][c] {
			case 1:
				cell = "<div class='token-wrap" + wrapCls + "'><div class='token red" + tokenCls + "'></div></div>"
			case 2:
				cell = "<div class='token-wrap" + wrapCls + "'><div class='token yellow" + tokenCls + "'></div></div>"
			}
			html += "<td data-col='" + strconv.Itoa(c) + "'>" + cell + "</td>"
		}
		html += "</tr>"
	}
	html += "</table>\n"
	html += "</div>" // end board-wrap
	html += "<div class='controls'><button name='reset' value='1'>Nouvelle partie</button>"
	if g.GameOver {
		html += "<button name='rematch' value='1'>Revanche</button>"
	}
	html += "</div></form>"

	// JS pour gérer le clic directement sur une colonne du plateau et la surbrillance au survol
	if !g.GameOver && !disableInterface {
		html += `<script>
		(function(){
			var form = document.getElementById('board-form');
			var colInput = document.getElementById('col-input');
			function setColHighlight(col, on){
				document.querySelectorAll('#board td[data-col="' + col + '"]').forEach(function(td){
					if(on){ td.classList.add('col-selected'); } else { td.classList.remove('col-selected'); }
				});
			}
			document.querySelectorAll('#board td').forEach(function(td){
				var col = td.getAttribute('data-col');
				if(col === null) return;
				td.addEventListener('mouseenter', function(){ setColHighlight(col, true); });
				td.addEventListener('mouseleave', function(){ setColHighlight(col, false); });
				td.addEventListener('click', function(){
					colInput.value = col;
					form.submit();
				});
			});
		})();
		</script>`
	}

	// Si c'est au tour de l'IA en mode Humain vs IA, on lance un fetch vers /ai-move après un délai
	if g.GameMode == ModeHumanVsAI && g.CurrentPlayer == 2 && !g.GameOver {
		// expose le délai en ms via data attribute et lance le fetch après le délai
		html += "<script>" +
			"(function(){ var delay=" + strconv.Itoa(aiDelayMs) + "; setTimeout(function(){ fetch('/ai-move', {method: 'POST'}).then(function(){ window.location.reload(); }); }, delay); })();" +
			"</script>"
	}

	return template.HTML(html)
}

// --- Template loading ---
var (
	pageTmpl  *template.Template
	startTmpl *template.Template
	winTmpl   *template.Template
	loseTmpl  *template.Template
	modeTmpl  *template.Template
)

func loadTemplates() error {
	var err error
	pageTmpl, err = template.ParseFiles("templates/game.html")
	if err != nil {
		return err
	}
	startTmpl, err = template.ParseFiles("templates/start.html")
	if err != nil {
		return err
	}
	winTmpl, err = template.ParseFiles("templates/win.html")
	if err != nil {
		return err
	}
	loseTmpl, err = template.ParseFiles("templates/lose.html")
	if err != nil {
		return err
	}
	modeTmpl, err = template.ParseFiles("templates/mode.html")
	return err
}

// --- Nouveau handler pour choisir le mode ---
func modeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		mode := r.FormValue("mode")
		username := r.FormValue("username")
		username2 := r.FormValue("username2")
		difficulty := r.FormValue("difficulty")
		skin := r.FormValue("skin") // Ajout du skin
		gamemode := r.FormValue("gamemode")
		ailevel := r.FormValue("ailevel")

		url := "/connect4?username=" + username + "&difficulty=" + difficulty + "&mode=" + mode + "&skin=" + skin + "&gamemode=" + gamemode
		if username2 != "" {
			url += "&username2=" + username2
		}
		if ailevel != "" {
			url += "&ailevel=" + ailevel
		}

		http.Redirect(w, r, url, http.StatusSeeOther)
		return
	}
	// On récupère tous les paramètres pour les garder dans le formulaire
	username := r.URL.Query().Get("username")
	username2 := r.URL.Query().Get("username2")
	difficulty := r.URL.Query().Get("difficulty")
	skin := r.URL.Query().Get("skin") // Ajout du skin
	gamemode := r.URL.Query().Get("gamemode")
	ailevel := r.URL.Query().Get("ailevel")

	modeTmpl.Execute(w, map[string]interface{}{
		"Username":   username,
		"Username2":  username2,
		"Difficulty": difficulty,
		"Skin":       skin, // Ajout du skin
		"GameMode":   gamemode,
		"AILevel":    ailevel,
	})
}

// --- Modifie startHandler pour rediriger vers /mode ---
func startHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		username := r.FormValue("username")
		username2 := r.FormValue("username2")
		difficulty := r.FormValue("difficulty")
		skin := r.FormValue("skin")
		gamemode := r.FormValue("gamemode")
		ailevel := r.FormValue("ailevel")

		url := "/mode?username=" + username + "&difficulty=" + difficulty + "&skin=" + skin + "&gamemode=" + gamemode
		if username2 != "" {
			url += "&username2=" + username2
		}
		if ailevel != "" {
			url += "&ailevel=" + ailevel
		}

		http.Redirect(w, r, url, http.StatusSeeOther)
		return
	}
	startTmpl.Execute(w, nil)
}

// --- Modifie handler pour prendre en compte le mode ---
func handler(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	defer mutex.Unlock()
	username := r.URL.Query().Get("username")
	username2 := r.URL.Query().Get("username2")
	difficulty := r.URL.Query().Get("difficulty")
	mode := r.URL.Query().Get("mode")
	skin := r.URL.Query().Get("skin") // Ajout du skin
	gamemodeStr := r.URL.Query().Get("gamemode")
	ailevelStr := r.URL.Query().Get("ailevel")

	if mode != "inverse" {
		mode = "normal"
	}

	// Parse GameMode
	var gameMode GameMode = ModeHumanVsHuman
	if gamemodeStr == "ai" {
		gameMode = ModeHumanVsAI
	}

	// Parse AILevel
	var aiLevel AILevel = AIEasy
	switch ailevelStr {
	case "medium":
		aiLevel = AIMedium
	case "hard":
		aiLevel = AIHard
	default:
		aiLevel = AIEasy
	}

	rows, cols, prefill := 6, 7, 0
	switch difficulty {
	case "easy":
		rows, cols, prefill = 6, 7, 0
	case "normal":
		rows, cols, prefill = 7, 8, 0
	case "hard":
		rows, cols, prefill = 8, 10, 7
	}

	// Normalise username2 pour le mode IA afin d'éviter une réinitialisation en boucle
	normUsername2 := username2
	if gameMode == ModeHumanVsAI && normUsername2 == "" {
		normUsername2 = "IA"
	}

	if game == nil || (username != "" && (game.Username != username || game.Username2 != normUsername2 || game.Difficulty != difficulty || game.Mode != mode || game.GameMode != gameMode || game.AILevel != aiLevel || game.Skin != skin)) {
		game = NewGame(rows, cols, prefill, difficulty, username, normUsername2, mode, skin, gameMode, aiLevel)
	}

	if r.Method == "POST" {
		r.ParseForm()
		if r.FormValue("reset") == "1" {
			game = nil
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		if r.FormValue("rematch") == "1" {
			game = NewGame(rows, cols, prefill, difficulty, username, normUsername2, mode, skin, gameMode, aiLevel)
		} else if colStr := r.FormValue("col"); colStr != "" {
			col, err := strconv.Atoi(colStr)
			if err == nil {
				game.DropToken(col)

				// En mode IA, ne joue PAS immédiatement ici.
				// Le client déclenchera le coup IA après un délai (aiDelayMs) via /ai-move.
			}
			// --- FIX: Redirect to avoid form resubmission on reload ---
			http.Redirect(w, r, r.URL.String(), http.StatusSeeOther)
			return
		}
	}

	// Prépare le message de fin si besoin
	endMessage := ""
	if game.GameOver {
		if game.Winner == 1 {
			name := game.Username1
			if name == "" {
				name = "Joueur 1"
			}
			endMessage = "🎉 Victoire de " + name + " !"
		} else if game.Winner == 2 {
			if game.GameMode == ModeHumanVsAI {
				endMessage = "🤖 L'IA a gagné !"
			} else {
				name := game.Username2
				if name == "" {
					name = "Joueur 2"
				}
				endMessage = "🎉 Victoire de " + name + " !"
			}
		} else {
			endMessage = "Match nul !"
		}
	}

	data := struct {
		BoardHTML     template.HTML
		CurrentPlayer int
		Winner        int
		GameOver      bool
		Gravity       Gravity
		Username      string
		Username1     string
		Username2     string
		Difficulty    string
		Rows          int
		Cols          int
		Mode          string
		GameMode      GameMode
		AILevel       AILevel
		Skin          string
		EndMessage    string
	}{
		BoardHTML:     renderBoard(game),
		CurrentPlayer: game.CurrentPlayer,
		Winner:        game.Winner,
		GameOver:      game.GameOver,
		Gravity:       game.Gravity,
		Username:      game.Username,
		Username1:     game.Username1,
		Username2:     game.Username2,
		Difficulty:    game.Difficulty,
		Rows:          game.Rows,
		Cols:          game.Cols,
		Mode:          game.Mode,
		GameMode:      game.GameMode,
		AILevel:       game.AILevel,
		Skin:          game.Skin,
		EndMessage:    endMessage,
	}
	pageTmpl.Execute(w, data)
}

func main() {
	// 1. Chargement des templates (comme sur ta photo)
	if err := loadTemplates(); err != nil {
		panic("Erreur chargement templates: " + err.Error())
	}

	// 2. Tes routes (comme sur ta photo)
	http.HandleFunc("/", startHandler)
	http.HandleFunc("/mode", modeHandler)
	http.HandleFunc("/ai-move", aiMoveHandler)
	http.HandleFunc("/connect4", handler)

	// 3. Gestion du CSS avec cache désactivé (comme sur ta photo)
	http.HandleFunc("/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		http.ServeFile(w, r, "style.css")
	})

	http.HandleFunc("/favicon.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		http.ServeFile(w, r, "favicon.svg")
	})

	// 5. GESTION DU PORT (Coolify)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Println("Server starting on port " + port)

	// 6. Lancement du serveur
	http.ListenAndServe(":"+port, nil)
}

// aiMoveHandler effectue le coup de l'IA lorsqu'il est appelé (endpoint POST)
func aiMoveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	if game == nil || game.GameMode != ModeHumanVsAI || game.GameOver || game.CurrentPlayer != 2 {
		// Rien à faire
		w.WriteHeader(http.StatusNoContent)
		return
	}

	aiCol := game.aiMove()
	if aiCol >= 0 {
		game.DropToken(aiCol)
	}

	// OK
	w.WriteHeader(http.StatusOK)
}
