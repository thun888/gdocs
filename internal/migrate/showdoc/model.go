package showdoc

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type DB struct {
	db *sql.DB
}

func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("打开 ShowDoc 数据库: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("读取 ShowDoc 数据库: %w", err)
	}
	return &DB{db: db}, nil
}

func (d *DB) Close() error { return d.db.Close() }

type Item struct {
	ItemID      int64
	Name        string
	Description string
	Username    string
	Password    string
	AddTime     int64
	Type        int
	Archived    bool
	Deleted     bool
}

type Catalog struct {
	CatID    int64
	ItemID   int64
	ParentID int64
	Name     string
	Sort     int
	AddTime  int64
}

type Page struct {
	PageID     int64
	AuthorUID  int64
	AuthorName string
	ItemID     int64
	CatID      int64
	Title      string
	Content    string
	Sort       int
	AddTime    int64
	Deleted    bool
	Draft      bool
}

type History struct {
	ID         int64
	PageID     int64
	AuthorUID  int64
	AuthorName string
	Title      string
	Content    string
	AddTime    int64
}

type User struct {
	UID      int64
	Username string
	Name     string
}

type Upload struct {
	FileID      int64
	Sign        string
	DisplayName string
	FileType    string
	RealURL     string
	AddTime     int64
}

func (d *DB) Items(ctx context.Context) ([]Item, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT item_id, item_name, item_description, username, password, addtime, item_type, is_archived, is_del
		FROM item ORDER BY item_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Item
	for rows.Next() {
		var item Item
		var name, description, username, password sql.NullString
		var itemType, archived, deleted sql.NullInt64
		if err := rows.Scan(&item.ItemID, &name, &description, &username, &password, &item.AddTime, &itemType, &archived, &deleted); err != nil {
			return nil, err
		}
		item.Name = name.String
		item.Description = description.String
		item.Username = username.String
		item.Password = password.String
		item.Type = int(itemType.Int64)
		item.Archived = archived.Int64 != 0
		item.Deleted = deleted.Int64 != 0
		out = append(out, item)
	}
	return out, rows.Err()
}

func (d *DB) Catalogs(ctx context.Context) ([]Catalog, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT cat_id, item_id, parent_cat_id, cat_name, s_number, addtime
		FROM catalog ORDER BY item_id, parent_cat_id, s_number, cat_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Catalog
	for rows.Next() {
		var item Catalog
		var name sql.NullString
		if err := rows.Scan(&item.CatID, &item.ItemID, &item.ParentID, &name, &item.Sort, &item.AddTime); err != nil {
			return nil, err
		}
		item.Name = name.String
		out = append(out, item)
	}
	return out, rows.Err()
}

func (d *DB) Pages(ctx context.Context) ([]Page, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT page_id, author_uid, author_username, item_id, cat_id, page_title, page_content,
		       s_number, addtime, is_del, is_draft
		FROM page ORDER BY item_id, cat_id, s_number, page_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Page
	for rows.Next() {
		var item Page
		var authorName, title, content sql.NullString
		var authorUID, itemID, catID, sort, deleted, draft sql.NullInt64
		if err := rows.Scan(&item.PageID, &authorUID, &authorName, &itemID, &catID, &title, &content,
			&sort, &item.AddTime, &deleted, &draft); err != nil {
			return nil, err
		}
		item.AuthorUID = authorUID.Int64
		item.AuthorName = authorName.String
		item.ItemID = itemID.Int64
		item.CatID = catID.Int64
		item.Title = title.String
		item.Content = content.String
		item.Sort = int(sort.Int64)
		item.Deleted = deleted.Int64 != 0
		item.Draft = draft.Int64 != 0
		out = append(out, item)
	}
	return out, rows.Err()
}

func (d *DB) Histories(ctx context.Context) ([]History, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT page_history_id, page_id, author_uid, author_username, page_title, page_content, addtime
		FROM page_history ORDER BY page_id, addtime, page_history_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []History
	for rows.Next() {
		var item History
		var authorName, title, content sql.NullString
		var authorUID sql.NullInt64
		if err := rows.Scan(&item.ID, &item.PageID, &authorUID, &authorName, &title, &content, &item.AddTime); err != nil {
			return nil, err
		}
		item.AuthorUID = authorUID.Int64
		item.AuthorName = authorName.String
		item.Title = title.String
		item.Content = content.String
		out = append(out, item)
	}
	return out, rows.Err()
}

func (d *DB) Users(ctx context.Context) ([]User, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT uid, username, name FROM user ORDER BY uid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var item User
		var username, name sql.NullString
		if err := rows.Scan(&item.UID, &username, &name); err != nil {
			return nil, err
		}
		item.Username = username.String
		item.Name = name.String
		out = append(out, item)
	}
	return out, rows.Err()
}

func (d *DB) Uploads(ctx context.Context) ([]Upload, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT file_id, sign, display_name, file_type, real_url, addtime
		FROM upload_file ORDER BY file_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Upload
	for rows.Next() {
		var item Upload
		var sign, name, fileType, realURL sql.NullString
		if err := rows.Scan(&item.FileID, &sign, &name, &fileType, &realURL, &item.AddTime); err != nil {
			return nil, err
		}
		item.Sign = sign.String
		item.DisplayName = name.String
		item.FileType = fileType.String
		item.RealURL = realURL.String
		out = append(out, item)
	}
	return out, rows.Err()
}
