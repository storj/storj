package utils // import github.com/cam-a/storj-client

import (
  "database/sql"
  "fmt"
  "time"

  "github.com/aleitner/piece-store/src"
)

// go routine to check ttl database for expired entries
// pass in DB and location of file for deletion
func DbChecker(db *sql.DB, dir string) {
	tickChan := time.NewTicker(time.Second * 5).C
	for {
		select {
		case <-tickChan:
			rows, err := db.Query(fmt.Sprintf("SELECT hash, expires FROM ttl WHERE expires < %d", time.Now().Unix()))
			if err != nil {
				fmt.Printf("Error: ", err.Error())
			}
			defer rows.Close()

			// iterate though selected rows
			// tried to wrap this inside (if rows != nil) but seems rows has value even if no entries meet condition. Thoughts?
			for rows.Next() {
				var expHash string
				var expires int64

				err = rows.Scan(&expHash, &expires)
				if err != nil {
					fmt.Printf("Error: ", err.Error())
					return
				}

				// delete file on local machine
				err = pstore.Delete(expHash, dir)
				if err != nil {
					fmt.Printf("Error: ", err.Error())
					return
				}
				fmt.Println("Deleted file: ", expHash)
			}

			// getting error when attempting to delete DB entry while inside it, so deleting outside for loop. Thoughts?
			_, err = db.Exec(fmt.Sprintf("DELETE FROM ttl WHERE expires < %d", time.Now().Unix()))
			if err != nil {
				fmt.Printf("Error: ", err.Error())
				return
			}
		}
	}
}
