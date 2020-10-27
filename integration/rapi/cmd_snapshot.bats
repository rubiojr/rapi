@test "rapi snapshot prints help" {
  run ./rapi snapshot
  [ "$status" -eq 0 ]
  [[ "$output" =~ "USAGE:" ]]
}

@test "rapi snapshot info without snapshots" {
  ./script/init-test-repo
  run ./rapi snapshot info
  [ "$status" -eq 1 ]
  [[ "$output" =~ 'no snapshot found' ]]
}

@test "rapi snapshot info with snapshots" {
  ./script/init-test-repo
  restic backup integration/fixtures > /dev/null
  run ./rapi snapshot info
  [ "$status" -eq 0 ]
  blobs=$(restic list blobs)
  info=$(restic cat config)
  info_id=$(echo $info | jq -r .id)
  info_pol=$(echo $info | jq -r .chunker_polynomial)
  read -r -d '' TEST_OUT <<EOF || true
Total Blob Count:    $(echo "$blobs"|wc -l)
Unique Files Size:   13 MB (deduped 0 B)
Total Files:         3
Unique Files:        3
Restore Size:        13 MB
EOF
  [[ "$(echo "$output" | grep -v Location)" =~ "$TEST_OUT" ]]
}