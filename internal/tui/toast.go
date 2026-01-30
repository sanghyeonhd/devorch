// Package tui provides toast notification system for CLI
package tui

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ToastVariant represents the type of toast notification
type ToastVariant string

const (
	ToastInfo    ToastVariant = "info"
	ToastSuccess ToastVariant = "success"
	ToastWarning ToastVariant = "warning"
	ToastError   ToastVariant = "error"
)

// Toast represents a toast notification
type Toast struct {
	ID        string
	Title     string
	Message   string
	Variant   ToastVariant
	Duration  time.Duration
	CreatedAt time.Time
	Dismissed bool
}

// ToastManager manages toast notifications
type ToastManager struct {
	toasts    []*Toast
	mu        sync.RWMutex
	theme     *Theme
	maxToasts int
	idCounter int
}

// NewToastManager creates a new toast manager
func NewToastManager(theme *Theme) *ToastManager {
	return &ToastManager{
		toasts:    make([]*Toast, 0),
		theme:     theme,
		maxToasts: 5, // Max visible toasts
	}
}

// Show displays a toast notification
func (tm *ToastManager) Show(variant ToastVariant, message string) *Toast {
	return tm.ShowWithTitle(variant, "", message)
}

// ShowWithTitle displays a toast notification with a title
func (tm *ToastManager) ShowWithTitle(variant ToastVariant, title, message string) *Toast {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.idCounter++
	toast := &Toast{
		ID:        fmt.Sprintf("toast-%d", tm.idCounter),
		Title:     title,
		Message:   message,
		Variant:   variant,
		Duration:  5 * time.Second,
		CreatedAt: time.Now(),
	}

	tm.toasts = append(tm.toasts, toast)

	// Trim old toasts
	if len(tm.toasts) > tm.maxToasts {
		tm.toasts = tm.toasts[len(tm.toasts)-tm.maxToasts:]
	}

	// Print immediately for CLI
	tm.printToast(toast)

	// Auto-dismiss after duration
	go func() {
		time.Sleep(toast.Duration)
		tm.Dismiss(toast.ID)
	}()

	return toast
}

// printToast prints a toast notification to the terminal
func (tm *ToastManager) printToast(toast *Toast) {
	var icon, style string

	switch toast.Variant {
	case ToastSuccess:
		icon = "✓"
		if tm.theme != nil {
			style = tm.theme.Success.Render(icon)
		} else {
			style = "\033[32m" + icon + "\033[0m" // Green
		}
	case ToastError:
		icon = "✗"
		if tm.theme != nil {
			style = tm.theme.Error.Render(icon)
		} else {
			style = "\033[31m" + icon + "\033[0m" // Red
		}
	case ToastWarning:
		icon = "⚠"
		if tm.theme != nil {
			style = tm.theme.Warning.Render(icon)
		} else {
			style = "\033[33m" + icon + "\033[0m" // Yellow
		}
	default: // Info
		icon = "ℹ"
		if tm.theme != nil {
			style = tm.theme.Accent.Render(icon)
		} else {
			style = "\033[34m" + icon + "\033[0m" // Blue
		}
	}

	// Build message
	var output strings.Builder
	output.WriteString("\n  ")
	output.WriteString(style)
	output.WriteString(" ")

	if toast.Title != "" {
		if tm.theme != nil {
			output.WriteString(tm.theme.Title.Render(toast.Title))
		} else {
			output.WriteString("\033[1m" + toast.Title + "\033[0m")
		}
		output.WriteString(": ")
	}

	output.WriteString(toast.Message)
	output.WriteString("\n")

	fmt.Print(output.String())
}

// Dismiss dismisses a toast by ID
func (tm *ToastManager) Dismiss(id string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for _, t := range tm.toasts {
		if t.ID == id {
			t.Dismissed = true
			break
		}
	}
}

// DismissAll dismisses all toasts
func (tm *ToastManager) DismissAll() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for _, t := range tm.toasts {
		t.Dismissed = true
	}
}

// GetActive returns all non-dismissed toasts
func (tm *ToastManager) GetActive() []*Toast {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	active := make([]*Toast, 0)
	for _, t := range tm.toasts {
		if !t.Dismissed {
			active = append(active, t)
		}
	}
	return active
}

// Clear removes all toasts
func (tm *ToastManager) Clear() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.toasts = make([]*Toast, 0)
}

// Convenience functions for quick toast creation

// Info shows an info toast
func (tm *ToastManager) Info(message string) *Toast {
	return tm.Show(ToastInfo, message)
}

// Success shows a success toast
func (tm *ToastManager) Success(message string) *Toast {
	return tm.Show(ToastSuccess, message)
}

// Warning shows a warning toast
func (tm *ToastManager) Warning(message string) *Toast {
	return tm.Show(ToastWarning, message)
}

// Error shows an error toast
func (tm *ToastManager) Error(message string) *Toast {
	return tm.Show(ToastError, message)
}

// ErrorWithTitle shows an error toast with title
func (tm *ToastManager) ErrorWithTitle(title, message string) *Toast {
	return tm.ShowWithTitle(ToastError, title, message)
}

// Global toast manager instance
var globalToastManager *ToastManager
var toastOnce sync.Once

// GetToastManager returns the global toast manager
func GetToastManager() *ToastManager {
	toastOnce.Do(func() {
		globalToastManager = NewToastManager(nil)
	})
	return globalToastManager
}

// SetToastTheme sets the theme for the global toast manager
func SetToastTheme(theme *Theme) {
	GetToastManager().theme = theme
}

// Quick global functions

// ShowToastInfo shows a global info toast
func ShowToastInfo(message string) {
	GetToastManager().Info(message)
}

// ShowToastSuccess shows a global success toast
func ShowToastSuccess(message string) {
	GetToastManager().Success(message)
}

// ShowToastWarning shows a global warning toast
func ShowToastWarning(message string) {
	GetToastManager().Warning(message)
}

// ShowToastError shows a global error toast
func ShowToastError(message string) {
	GetToastManager().Error(message)
}
