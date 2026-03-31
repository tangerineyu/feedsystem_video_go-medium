package main

import (
    "context"
    "database/sql"
    "flag"
    "fmt"
    "log"
    "math/rand"
    "strings"
    "time"

    "github.com/brianvoe/gofakeit/v7"
    _ "github.com/go-sql-driver/mysql"
    "golang.org/x/crypto/bcrypt"
)

func main() {
    var (
        dsn         = flag.String("dsn", "root:jzy@tcp(127.0.0.1:3306)/feedsystem?charset=utf8mb4&parseTime=True&loc=Local", "MySQL DSN")
        users       = flag.Int("users", 10000, "new users")
        videos      = flag.Int("videos", 200000, "new videos")
        likes       = flag.Int("likes", 3000000, "new likes (duplicates auto ignored)")
        comments    = flag.Int("comments", 1000000, "new comments")
        follows     = flag.Int("follows", 500000, "new follows (duplicates auto ignored)")
        batch       = flag.Int("batch", 1000, "batch size")
        seed        = flag.Int64("seed", time.Now().UnixNano(), "random seed")
        truncateAll = flag.Bool("truncate", false, "truncate tables before seeding")
    )
    flag.Parse()

    r := rand.New(rand.NewSource(*seed))
    gofakeit.Seed(*seed)

    db, err := sql.Open("mysql", *dsn)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    ctx := context.Background()
    if err := db.PingContext(ctx); err != nil {
        log.Fatal(err)
    }

    log.Printf("connected, seed=%d", *seed)

    if *truncateAll {
        if err := truncateTables(ctx, db); err != nil {
            log.Fatal(err)
        }
        log.Println("tables truncated")
    }

    baseAccountID := mustMaxID(ctx, db, "accounts")
    insertAccounts(ctx, db, baseAccountID, *users, *batch)

    baseVideoID := mustMaxID(ctx, db, "videos")
    insertVideos(ctx, db, baseVideoID, baseAccountID, *users, *videos, *batch, r)

    insertLikes(ctx, db, baseVideoID, *videos, baseAccountID, *users, *likes, *batch, r)
    insertComments(ctx, db, baseVideoID, *videos, baseAccountID, *users, *comments, *batch, r)
    insertFollows(ctx, db, baseAccountID, *users, *follows, *batch, r)

    rebuildVideoCounters(ctx, db)

    log.Println("done")
}

func truncateTables(ctx context.Context, db *sql.DB) error {
    stmts := []string{
        "SET FOREIGN_KEY_CHECKS=0",
        "TRUNCATE TABLE likes",
        "TRUNCATE TABLE comments",
        "TRUNCATE TABLE socials",
        "TRUNCATE TABLE videos",
        "TRUNCATE TABLE accounts",
        "TRUNCATE TABLE outbox_msgs",
        "SET FOREIGN_KEY_CHECKS=1",
    }
    for _, s := range stmts {
        if _, err := db.ExecContext(ctx, s); err != nil {
            return fmt.Errorf("exec %q: %w", s, err)
        }
    }
    return nil
}

func mustMaxID(ctx context.Context, db *sql.DB, table string) uint64 {
    var id sql.NullInt64
    q := fmt.Sprintf("SELECT MAX(id) FROM %s", table)
    if err := db.QueryRowContext(ctx, q).Scan(&id); err != nil {
        log.Fatalf("max id %s: %v", table, err)
    }
    if !id.Valid || id.Int64 < 0 {
        return 0
    }
    return uint64(id.Int64)
}

func insertAccounts(ctx context.Context, db *sql.DB, baseID uint64, n, batch int) {
    if n <= 0 {
        return
    }
    passHash, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
    if err != nil {
        log.Fatal(err)
    }
    for start := 0; start < n; start += batch {
        end := min(start+batch, n)
        vals := make([]string, 0, end-start)
        args := make([]any, 0, (end-start)*4)
        for i := start; i < end; i++ {
            aid := baseID + uint64(i) + 1
            username := fmt.Sprintf("user_%08d", aid)
            vals = append(vals, "(?,?,?,?)")
            args = append(args, username, string(passHash), "", "")
        }
        q := "INSERT INTO accounts (username,password,token,refresh_token_hash) VALUES " + strings.Join(vals, ",")
        if _, err := db.ExecContext(ctx, q, args...); err != nil {
            log.Fatalf("insert accounts batch [%d,%d): %v", start, end, err)
        }
    }
    log.Printf("accounts +%d", n)
}

func insertVideos(ctx context.Context, db *sql.DB, baseVideoID, baseAccountID uint64, userN, n, batch int, r *rand.Rand) {
    if n <= 0 || userN <= 0 {
        return
    }
    for start := 0; start < n; start += batch {
        end := min(start+batch, n)
        vals := make([]string, 0, end-start)
        args := make([]any, 0, (end-start)*9)

        for i := start; i < end; i++ {
            authorID := baseAccountID + 1 + uint64(r.Intn(userN))
            username := fmt.Sprintf("user_%08d", authorID)

            vals = append(vals, "(?,?,?,?,?,?,?,?,?)")
            args = append(args,
                authorID,
                username,
                clip(gofakeit.Sentence(3)),
                clip(gofakeit.Sentence(8)),
                "https://cdn.example.com/video/"+gofakeit.UUID()+".mp4",
                "https://cdn.example.com/cover/"+gofakeit.UUID()+".jpg",
                time.Now().Add(-time.Duration(r.Intn(30*24))*time.Hour),
                0, // likes_count
                0, // popularity
            )
        }

        q := "INSERT INTO videos (author_id,username,title,description,play_url,cover_url,create_time,likes_count,popularity) VALUES " + strings.Join(vals, ",")
        if _, err := db.ExecContext(ctx, q, args...); err != nil {
            log.Fatalf("insert videos batch [%d,%d): %v", start, end, err)
        }
    }
    log.Printf("videos +%d (baseVideoID=%d)", n, baseVideoID)
}

func insertLikes(ctx context.Context, db *sql.DB, baseVideoID uint64, videoN int, baseAccountID uint64, userN int, n, batch int, r *rand.Rand) {
    if n <= 0 || videoN <= 0 || userN <= 0 {
        return
    }
    zipf := rand.NewZipf(r, 1.2, 3, uint64(videoN-1)) // 热点分布
    for start := 0; start < n; start += batch {
        end := min(start+batch, n)
        vals := make([]string, 0, end-start)
        args := make([]any, 0, (end-start)*3)
        now := time.Now()

        for i := start; i < end; i++ {
            videoID := baseVideoID + 1 + zipf.Uint64()
            accountID := baseAccountID + 1 + uint64(r.Intn(userN))
            vals = append(vals, "(?,?,?)")
            args = append(args, videoID, accountID, now.Add(-time.Duration(r.Intn(7*24))*time.Hour))
        }

        q := "INSERT IGNORE INTO likes (video_id,account_id,created_at) VALUES " + strings.Join(vals, ",")
        if _, err := db.ExecContext(ctx, q, args...); err != nil {
            log.Fatalf("insert likes batch [%d,%d): %v", start, end, err)
        }
    }
    log.Printf("likes attempted +%d", n)
}

func insertComments(ctx context.Context, db *sql.DB, baseVideoID uint64, videoN int, baseAccountID uint64, userN int, n, batch int, r *rand.Rand) {
    if n <= 0 || videoN <= 0 || userN <= 0 {
        return
    }
    zipf := rand.NewZipf(r, 1.15, 3, uint64(videoN-1))
    for start := 0; start < n; start += batch {
        end := min(start+batch, n)
        vals := make([]string, 0, end-start)
        args := make([]any, 0, (end-start)*5)
        now := time.Now()

        for i := start; i < end; i++ {
            videoID := baseVideoID + 1 + zipf.Uint64()
            authorID := baseAccountID + 1 + uint64(r.Intn(userN))
            username := fmt.Sprintf("user_%08d", authorID)

            vals = append(vals, "(?,?,?,?,?)")
            args = append(args,
                username,
                videoID,
                authorID,
                clip(gofakeit.Sentence(10)),
                now.Add(-time.Duration(r.Intn(7*24))*time.Hour),
            )
        }

        q := "INSERT INTO comments (username,video_id,author_id,content,created_at) VALUES " + strings.Join(vals, ",")
        if _, err := db.ExecContext(ctx, q, args...); err != nil {
            log.Fatalf("insert comments batch [%d,%d): %v", start, end, err)
        }
    }
    log.Printf("comments +%d", n)
}

func insertFollows(ctx context.Context, db *sql.DB, baseAccountID uint64, userN int, n, batch int, r *rand.Rand) {
    if n <= 0 || userN <= 1 {
        return
    }
    for start := 0; start < n; start += batch {
        end := min(start+batch, n)
        vals := make([]string, 0, end-start)
        args := make([]any, 0, (end-start)*2)

        for i := start; i < end; i++ {
            follower := baseAccountID + 1 + uint64(r.Intn(userN))
            vlogger := baseAccountID + 1 + uint64(r.Intn(userN))
            if follower == vlogger {
                if vlogger > baseAccountID+1 {
                    vlogger--
                } else {
                    vlogger++
                }
            }
            vals = append(vals, "(?,?)")
            args = append(args, follower, vlogger)
        }

        q := "INSERT IGNORE INTO socials (follower_id,vlogger_id) VALUES " + strings.Join(vals, ",")
        if _, err := db.ExecContext(ctx, q, args...); err != nil {
            log.Fatalf("insert follows batch [%d,%d): %v", start, end, err)
        }
    }
    log.Printf("follows attempted +%d", n)
}

func rebuildVideoCounters(ctx context.Context, db *sql.DB) {
    q := `
UPDATE videos v
LEFT JOIN (
    SELECT video_id, COUNT(*) cnt
    FROM likes
    GROUP BY video_id
) l ON l.video_id = v.id
LEFT JOIN (
    SELECT video_id, COUNT(*) cnt
    FROM comments
    GROUP BY video_id
) c ON c.video_id = v.id
SET
    v.likes_count = COALESCE(l.cnt, 0),
    v.popularity = COALESCE(l.cnt, 0) * 2 + COALESCE(c.cnt, 0)
`
    if _, err := db.ExecContext(ctx, q); err != nil {
        log.Fatalf("rebuild counters: %v", err)
    }
    log.Println("videos.likes_count/popularity rebuilt")
}

func clip(s string) string {
    if len(s) <= 240 {
        return s
    }
    return s[:240]
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}