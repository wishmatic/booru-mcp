package booru

import (
	"context"
	"net/http"
	"testing"
)

func TestSearchTagsMapsAndDropsEmptyNames(t *testing.T) {
	client := newTestClient(t, 100, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"name":"","category":0,"post_count":14115639},
			{"name":"Blue Eyes","category":0,"post_count":900},
			{"name":"somebody","category":1,"post_count":500},
			{"name":"a_series","category":3,"post_count":400},
			{"name":"someone","category":4,"post_count":300},
			{"name":"highres","category":5,"post_count":200}
		]`))
	})

	page, err := client.SearchTags(context.Background(), TagQuery{Search: "blue", Limit: 10})
	if err != nil {
		t.Fatalf("SearchTags() error: %v", err)
	}

	want := []Tag{
		{Name: "blue_eyes", Category: CategoryGeneral, Count: 900},
		{Name: "somebody", Category: CategoryArtist, Count: 500},
		{Name: "a_series", Category: CategoryCopyright, Count: 400},
		{Name: "someone", Category: CategoryCharacter, Count: 300},
		{Name: "highres", Category: CategoryMeta, Count: 200},
	}

	if len(page.Tags) != len(want) {
		t.Fatalf("tags = %+v, want %d with the empty name dropped", page.Tags, len(want))
	}

	for i, tag := range want {
		if page.Tags[i] != tag {
			t.Errorf("tags[%d] = %+v, want %+v", i, page.Tags[i], tag)
		}
	}
}
