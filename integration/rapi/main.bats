@test "rapi prints help" {
  run ./rapi
  [ "$status" -eq 0 ]
  [[ "$output" =~ "USAGE:" ]]
}
