package video

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestSearchVideoRequestCompiles(t *testing.T) {
	t.Parallel()

	req := SearchVideoRequest{
		Keyword:    "cat",
		Limit:      10,
		LatestTime: 1700000000000,
	}

	if req.Keyword != "cat" || req.Limit != 10 || req.LatestTime != 1700000000000 {
		t.Fatalf("unexpected request: %+v", req)
	}
}

func TestVideoServiceSearchRejectsEmptyKeyword(t *testing.T) {
	t.Parallel()

	svc := NewVideoService(nil, nil, nil)
	_, err := svc.Search(context.Background(), "   ", 10, time.Time{})
	if err == nil {
		t.Fatalf("expected empty keyword error")
	}
	if !strings.Contains(err.Error(), "keyword") {
		t.Fatalf("expected keyword error, got %q", err.Error())
	}
}

func TestBuildVideoSearchTerms(t *testing.T) {
	t.Parallel()

	terms := buildVideoSearchTerms(" Go 视频 Go ", "MariaDB 搜索优化", "alice")
	want := []string{"go", "视频", "mariadb", "搜索优化", "alice"}

	if !reflect.DeepEqual(terms, want) {
		t.Fatalf("terms mismatch\nwant: %#v\n got: %#v", want, terms)
	}
}

func TestVideoRepositorySearchUsesTermIndex(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "root:pass@tcp(localhost:3306)/feedsystem?charset=utf8mb4&parseTime=True&loc=Local",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open dry-run db: %v", err)
	}

	repo := NewVideoRepository(db)
	stmt := repo.searchVideoIDQuery(context.Background(), db, []string{"go"}, time.Unix(1700000000, 0)).
		Limit(11).
		Find(&[]searchVideoIDRow{}).Statement
	sql := stmt.SQL.String()

	if !strings.Contains(sql, "video_search_terms") {
		t.Fatalf("expected search to use video_search_terms, got SQL: %s", sql)
	}
	if strings.Contains(sql, "LIKE") || strings.Contains(sql, "videos") {
		t.Fatalf("expected no LIKE scan on videos, got SQL: %s", sql)
	}
}
