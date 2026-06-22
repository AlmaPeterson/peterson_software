package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// Redeploy pulls the latest commit from git and exits the process so a
// supervisor (systemd, configured with Restart=always) brings it back up
// running the new code. It only exits if the pull actually succeeded —
// if git fails (conflicts, network, etc.) the current process keeps running
// and the error is reported back instead.
func Redeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cmd := exec.Command("git", "pull", "--ff-only")
	cmd.Dir = ".."
	output, err := cmd.CombinedOutput()

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":  err.Error(),
			"output": string(output),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"output": string(output)})
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Give the response a moment to actually reach the client before the
	// process disappears.
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
}
