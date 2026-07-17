package logging

import (
	"testing"

	"github.com/vkh/spacemosquito/pkg/logger"
)

func TestSugar_Enabled_nil(t *testing.T) {
	var s Sugar
	if s.Enabled() {
		t.Fatal("zero-value Sugar should not be enabled")
	}
}

func TestSugar_Enabled_withLogger(t *testing.T) {
	zl, err := logger.NewProduction(nil)
	if err != nil {
		t.Fatal(err)
	}
	s := New("test", zl)
	if !s.Enabled() {
		t.Fatal("Sugar with logger should be enabled")
	}
}
