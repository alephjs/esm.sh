#!/bin/bash

mkdir -p ~/.ssh
echo "${DEPLOY_SSH_PRIVATE_KEY}" >> ~/.ssh/id_ed25519
chmod 600 ~/.ssh/id_ed25519
ssh-keyscan $DEPLOY_HOST >> ~/.ssh/known_hosts
echo "Host esm.sh" >> ~/.ssh/config
echo "  HostName ${DEPLOY_HOST}" >> ~/.ssh/config
echo "  Port ${DEPLOY_SSH_PORT}" >> ~/.ssh/config
echo "  User ${DEPLOY_SSH_USER}" >> ~/.ssh/config
echo "  IdentityFile ~/.ssh/id_ed25519" >> ~/.ssh/config
echo "  IdentitiesOnly yes" >> ~/.ssh/config

echo "--- building server..."
go build -ldflags="-s -w -X 'github.com/esm-dev/esm.sh/server.VERSION=${SERVER_VERSION}'" -o esmd $(dirname $0)/../server/cmd/main.go
if [ "$?" != "0" ]; then
  exit 1
fi
du -h esmd

echo "--- uploading server build..."
tar -czf esmd.tar.gz esmd
scp esmd.tar.gz esm.sh:/tmp/esmd.tar.gz
if [ "$?" != "0" ]; then
  exit 1
fi

echo "--- installing server..."
ssh esm.sh << EOF
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

  ufw version
  if [ "\$?" == "0" ]; then
    ufw allow http
  fi

  configfile=/etc/esmd/config.json
  servicefile=/etc/systemd/system/esmd.service
  reload=no
  if [ ! -f \$servicefile ]; then
    addgroup esm
    adduser --ingroup esm --home /esm --disabled-login --disabled-password --gecos "" esm
    if [ "\$?" != "0" ]; then
      echo "Failed to add user 'esm'"
      exit 1
    fi
    mkdir /etc/esmd
    echo "[Unit]" >> \$servicefile
    echo "Description=esm.sh service" >> \$servicefile
    echo "After=network.target" >> \$servicefile
    echo "StartLimitIntervalSec=0" >> \$servicefile
    echo "[Service]" >> \$servicefile
    echo "Type=simple" >> \$servicefile
    echo "ExecStart=/usr/local/bin/esmd --config=\$configfile" >> \$servicefile
    echo "WorkingDirectory=/esm" >> \$servicefile
    echo "Group=esm" >> \$servicefile
    echo "User=esm" >> \$servicefile
    echo "AmbientCapabilities=CAP_NET_BIND_SERVICE" >> \$servicefile
    echo "Restart=always" >> \$servicefile
    echo "RestartSec=5" >> \$servicefile
    echo "Environment=\"ESMDIR=/esm\"" >> \$servicefile
    echo "[Install]" >> \$servicefile
    echo "WantedBy=multi-user.target" >> \$servicefile
    reload=yes
  else
    systemctl stop esmd.service
    echo "Stopped esmd.service."
  fi

  rm -f \$configfile
  if [ "$SERVER_CONFIG" != "" ]; then
    echo '${SERVER_CONFIG}' >> \$configfile
  else
    echo "{}" >> \$configfile
  fi

  if [ "$RESET_ON_DEPLOY" == "yes" ]; then
    mkdir -p /tmp/.esm
    mv -f /esm/* /tmp/.esm
    nohup rm -rf /tmp/.esm &
  fi

  mv -f esmd /usr/local/bin/esmd

  if [ "\$reload" == "yes" ]; then
    systemctl daemon-reload
    systemctl enable esmd.service
  fi

  systemctl start esmd.service
  echo "Started esmd.service."
EOF
if [ "$?" != "0" ]; then
  exit 1
fi
