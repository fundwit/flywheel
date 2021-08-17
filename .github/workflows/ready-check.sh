#! /bin/sh
set +e
echo "wating for ready"

failureThreshold=20
initialDelaySeconds=1
periodSeconds=5

for i in $(seq 1 $failureThreshold); do
  if [[ "$i" == 1 ]]; then
    sleep $initialDelaySeconds
  else
    sleep $periodSeconds
  fi

  echo "cmd: '$@'"
  $@

  if [[ "$?" == "0" ]]; then
    echo "service ready"
    exit 0
  else
    echo "service not ready yet"
  fi
done

exit 1