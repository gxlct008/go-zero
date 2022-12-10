package health

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

const probeName = "probe"

func TestHealthManager(t *testing.T) {
	hm := NewHealthManager(probeName)
	assert.False(t, hm.IsReady())

	hm.MarkReady()
	assert.True(t, hm.IsReady())

	hm.MarkNotReady()
	assert.False(t, hm.IsReady())

	t.Run("concurrent should works", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(10)
		for i := 0; i < 10; i++ {
			go func() {
				hm.MarkReady()
				wg.Done()
			}()
		}
		wg.Wait()
		assert.True(t, hm.IsReady())
	})
}

func TestComboHealthManager(t *testing.T) {
	t.Run("base", func(t *testing.T) {
		chm := newComboHealthManager()
		hm1 := NewHealthManager(probeName)
		hm2 := NewHealthManager(probeName + "2")

		assert.True(t, chm.IsReady())
		chm.addProbe(hm1)
		chm.addProbe(hm2)
		assert.False(t, chm.IsReady())
		hm1.MarkReady()
		assert.False(t, chm.IsReady())
		hm2.MarkReady()
		assert.True(t, chm.IsReady())
	})

	t.Run("concurrent add probes", func(t *testing.T) {
		chm2 := newComboHealthManager()

		var wg sync.WaitGroup
		wg.Add(10)
		for i := 0; i < 10; i++ {
			go func() {
				hm := NewHealthManager(probeName)
				hm.MarkReady()
				chm2.addProbe(hm)
				wg.Done()
			}()
		}
		wg.Wait()
		assert.True(t, chm2.IsReady())
	})

	t.Run("markReady and markNotReady", func(t *testing.T) {
		chm2 := newComboHealthManager()

		for i := 0; i < 10; i++ {
			hm := NewHealthManager(probeName)
			chm2.addProbe(hm)
		}
		assert.False(t, chm2.IsReady())

		chm2.MarkReady()
		assert.True(t, chm2.IsReady())

		chm2.MarkNotReady()
		assert.False(t, chm2.IsReady())
	})
}

func TestAddGlobalProbes(t *testing.T) {
	cleanupForTest(t)

	t.Run("concurrent add probes", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(10)
		for i := 0; i < 10; i++ {
			go func() {
				hm := NewHealthManager(probeName)
				hm.MarkReady()
				AddProbe(hm)
				wg.Done()
			}()
		}
		wg.Wait()
		assert.True(t, defaultHealthManager.IsReady())
	})
}

func TestCreateHttpHandler(t *testing.T) {
	cleanupForTest(t)
	srv := httptest.NewServer(CreateHttpHandler())
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	assert.Nil(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	hm := NewHealthManager(probeName)
	defaultHealthManager.addProbe(hm)

	resp, err = http.Get(srv.URL)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	content, _ := io.ReadAll(resp.Body)
	assert.True(t, strings.HasPrefix(string(content), "Service Unavailable"))
	_ = resp.Body.Close()

	hm.MarkReady()
	resp, err = http.Get(srv.URL)
	assert.Nil(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func cleanupForTest(t *testing.T) {
	t.Cleanup(func() {
		defaultHealthManager = &comboHealthManager{}
	})
}