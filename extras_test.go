package jsonapi

import "testing"

func TestAddRelationshipLink(t *testing.T) {
	extras := new(ApiExtras)

	extras.AddRelationshipLink("next", "posts", "posts", "blogs", "https://localhost:9000/api/v1/blogs/posts?blog_id={blog.}&id={post.id}")

	if len(extras.RelationshipLinks) != 1 {
		t.Fatalf("did not add link")
	}
}

func TestAddRootLink(t *testing.T) {
	extras := new(ApiExtras)

	extras.AddRootLink("next", "https://localhost:9000/api/v1/blogs/posts?blog_id={blog.}&id={post.id}")

	if len(extras.RootLinks) != 1 {
		t.Fatalf("did not add link")
	}
}
