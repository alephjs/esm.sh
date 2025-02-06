#!/bin/bash

init="no"
host="$1"
if [ "$host" == "--init" ]; then
  init="yes"
  host="$2"
fi

config=""
if [ "$init" == "yes" ]; then
  read -p "? server configuration (JSON): " v
  if [ "$v" != "" ]; then
    config="$v"
  fi
fi

if [ "$host" == "" ]; then
  read -p "? deploy to (domain or IP): " v
  if [ "$v" != "" ]; then
    host="$v"
  fi
fi

if [ "$host" == "" ]; then
  echo "missing host"
  exit
fi

read -p "? host os (default is \"linux\"): " goos
read -p "? host architecture (default is \"amd64\"): " goarch
if [ "$goos" == "" ]; then
  goos="linux"
fi
if [ "$goarch" == "" ]; then
  goarch="amd64"
fi

user="root"
read -p "? login user (default is \"root\"): " v
if [ "$v" != "" ]; then
  user="$v"
fi

sshPort="22"
read -p "? ssh port (default is 22): " v
if [ "$v" != "" ]; then
  sshPort="$v"
fi

echo "--- building(${goos}_$goarch)..."
export GOOS=$goos
export GOARCH=$goarch
go build -ldflags="-s -w" -o esmd $(dirname $0)/../server/cmd/main.go
if [ "$?" != "0" ]; then
  exit
fi

echo "--- uploading..."
tar -czf esmd.tar.gz esmd
scp -P $sshPort esmd.tar.gz $user@$host:/tmp/esmd.tar.gz
if [ "$?" != "0" ]; then
  rm -f esmd
  rm -f esmd.tar.gz
  exit
fi

echo "--- installing..."
ssh -p $sshPort ${user}@${host} << EOF
  cd /tmp
  tar -xzf esmd.tar.gz
  if [ "\$?" != "0" ]; then
    exit 1
  fi
  chmod +x esmd
  rm -f esmd.tar.gz

  git version
  if [ "\$?" == "127" ]; then
    apt-get update
    apt-get install -y git
  fi

  if [ "$init" == "yes" ]; then
    servicefile=/etc/systemd/system/esmd.service
    if [ -f \$servicefile ]; then
      rm -f servicefile
    fi
    addgroup esm
    adduser --ingroup esm --home=/esm --disabled-login --disabled-password --gecos "" esm
    if [ "\$?" != "0" ]; then
      echo "Failed to add user 'esm'"
      exit 1
    fi
    ufw version
    if [ "\$?" == "0" ]; then
      ufw allow http
    fi
    echo "[Unit]" >> \$servicefile
    echo "Description=esm.sh service" >> \$servicefile
    echo "After=network.target" >> \$servicefile
    echo "StartLimitIntervalSec=0" >> \$servicefile
    echo "[Service]" >> \$servicefile
    echo "Type=simple" >> \$servicefile
    if [ "$config" != "" ]; then
      configfile=/etc/esmd/config.json
      mkdir -p /etc/esmd
      rm -f \$configfile
      echo '$config' >> \$configfile
      echo "ExecStart=/usr/local/bin/esmd --config=\$configfile" >> \$servicefile
    else
      echo "ExecStart=/usr/local/bin/esmd" >> \$servicefile
    fi
    echo "WorkingDirectory=/esm" >> \$servicefile
    echo "Group=esm" >> \$servicefile
    echo "User=esm" >> \$servicefile
    echo "AmbientCapabilities=CAP_NET_BIND_SERVICE" >> \$servicefile
    echo "Restart=always" >> \$servicefile
    echo "RestartSec=5" >> \$servicefile
    echo "Environment=\"ESMDIR=/esm\"" >> \$servicefile
    echo "[Install]" >> \$servicefile
    echo "WantedBy=multi-user.target" >> \$servicefile
  else
    systemctl stop esmd.service
    echo "Stopped esmd.service."
  fi

  mv -f esmd /usr/local/bin/esmd

  if [ "$init" == "yes" ]; then
    systemctl daemon-reload
    systemctl enable esmd.service
  fi

  systemctl start esmd.service
  echo "Started esmd.service."
EOF

rm -f esmd
rm -f esmd.tar.gz
