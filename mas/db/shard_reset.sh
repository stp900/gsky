#!/bin/bash

here="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
shard=$1

(cd "$here" && runuser postgres -c 'psql -v ON_ERROR_STOP=1 -A -t -q -d nci' <<EOD

set role nci;
set search_path to ${shard};

truncate paths;
truncate metadata;

EOD
)
