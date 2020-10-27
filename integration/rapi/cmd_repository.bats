@test "rapi repository prints help" {
  run ./rapi repository
  [ "$status" -eq 0 ]
  [[ "$output" =~ "USAGE:" ]]
}

@test "rapi repository id prints the repo ID" {
  ./script/init-test-repo
  run ./rapi repository id
  repo_id=$(restic cat config | jq -r .id)
  [ "$status" -eq 0 ]
  [[ "$output" =~ "$repo_id" ]]
}

@test "rapi repository info when empty" {
  run ./rapi repository info
  info=$(restic cat config)
  info_id=$(echo $info | jq -r .id)
  info_pol=$(echo $info | jq -r .chunker_polynomial)
  [ "$status" -eq 0 ]
  read -r -d '' TEST_OUT <<EOF || true
ID:                  $info_id
Chunker polynomial:  0x$info_pol
Repository version:  1
Packs:               0
Tree blobs:          0
Data blobs:          0
EOF
  [[ "$(echo "$output" | grep -v Location)" =~ "$TEST_OUT" ]]
}

@test "rapi repository info when full" {
  backup=$(restic backup integration/fixtures)
  blobs=$(restic list blobs)
  tree_blobs=$(echo "$blobs"|grep ^tree|wc -l)
  data_blobs=$(echo "$blobs"|grep ^data|wc -l)
  packs=$(restic list packs|wc -l)
  run ./rapi repository info
  info=$(restic cat config)
  info_id=$(echo $info | jq -r .id)
  info_pol=$(echo $info | jq -r .chunker_polynomial)
  [ "$status" -eq 0 ]
  read -r -d '' TEST_OUT <<EOF || true
ID:                  $info_id
Chunker polynomial:  0x$info_pol
Repository version:  1
Packs:               $packs
Tree blobs:          $tree_blobs
Data blobs:          $data_blobs
EOF
  [[ "$(echo "$output" | grep -v Location)" =~ "$TEST_OUT" ]]
}