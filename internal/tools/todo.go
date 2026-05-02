package tools

import (
	"fmt"
	"strings"
	"sync"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// TodoStore is the in-process backing for the agent's todo list.
// Slice 3 keeps it ephemeral (one store per Cogo process); persistence
// across sessions can be added when transcripts land in Slice 5.
type TodoStore struct {
	mu    sync.Mutex
	items []TodoItem
}

// TodoItem is one entry in the agent's plan.
type TodoItem struct {
	ID     int    `json:"id"`
	Status string `json:"status"`           // "pending" | "in_progress" | "completed"
	Text   string `json:"text"`
}

// NewTodoStore returns a fresh, empty store.
func NewTodoStore() *TodoStore { return &TodoStore{} }

type todoArgs struct {
	Action string `json:"action" jsonschema:"one of: list, add, set_status, clear"`
	Text   string `json:"text,omitempty" jsonschema:"item text (for add)"`
	ID     int    `json:"id,omitempty" jsonschema:"item id (for set_status)"`
	Status string `json:"status,omitempty" jsonschema:"new status: pending|in_progress|completed (for set_status)"`
}

type todoResult struct {
	Items []TodoItem `json:"items"`
}

func todoFunc(store *TodoStore) functiontool.Func[todoArgs, todoResult] {
	return func(_ tool.Context, in todoArgs) (todoResult, error) {
		store.mu.Lock()
		defer store.mu.Unlock()
		switch strings.ToLower(in.Action) {
		case "list", "":
			return todoResult{Items: copyItems(store.items)}, nil
		case "add":
			if strings.TrimSpace(in.Text) == "" {
				return todoResult{}, fmt.Errorf("todo: text is required for add")
			}
			id := len(store.items) + 1
			store.items = append(store.items, TodoItem{ID: id, Status: "pending", Text: in.Text})
			return todoResult{Items: copyItems(store.items)}, nil
		case "set_status":
			s := strings.ToLower(in.Status)
			if s != "pending" && s != "in_progress" && s != "completed" {
				return todoResult{}, fmt.Errorf("todo: status must be pending|in_progress|completed")
			}
			for i := range store.items {
				if store.items[i].ID == in.ID {
					store.items[i].Status = s
					return todoResult{Items: copyItems(store.items)}, nil
				}
			}
			return todoResult{}, fmt.Errorf("todo: id %d not found", in.ID)
		case "clear":
			store.items = nil
			return todoResult{Items: nil}, nil
		default:
			return todoResult{}, fmt.Errorf("todo: unknown action %q", in.Action)
		}
	}
}

func copyItems(in []TodoItem) []TodoItem {
	out := make([]TodoItem, len(in))
	copy(out, in)
	return out
}
