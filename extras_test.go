package jsonapi

import "testing"

func TestAddLinkConfiguration(t *testing.T) {
	extras := new(ApiExtras)

	extras.AddLink("next", "post", []string{"blog"}, "https://localhost:9000/api/v1/blogs/posts?blog_id={blog.}&id={post.id}")

	if len(extras.Links) != 1 {
		t.Fatalf("did not add link")
	}
}
