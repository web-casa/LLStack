{{CUSTOM_HEAD}}
docRoot                   {{DOC_ROOT}}
vhDomain                  {{DOMAIN}}
{{ALIAS_CONF}}
enableGzip                1
enableBr                  1

index  {
  useServer               0
  indexFiles               index.php, index.html
}

scripthandler  {
  {{CUSTOM_HANDLER}}
  add                     lsapi:lsphp{{PHP_SHORT}} php
}

rewrite  {
  enable                  1
  autoLoadHtaccess        1
  logLevel                0
  {{CUSTOM_REWRITE}}
}

accesslog $SERVER_ROOT/logs/{{DOMAIN}}.access.log {
  useServer               0
  logFormat               "%h %l %u %t \"%r\" %>s %b \"%{Referer}i\" \"%{User-Agent}i\""
  logHeaders              5
  rollingSize             100M
  keepDays                30
  compressArchive         1
}

accesslog $SERVER_ROOT/logs/{{DOMAIN}}.bytes {
  useServer               0
  logFormat               %O %I
  rollingSize             0
}

errorlog $SERVER_ROOT/logs/{{DOMAIN}}.error.log {
  useServer               0
  logLevel                ERROR
  rollingSize             10M
}

module cache {
  storagePath             /usr/local/lsws/cachedata/{{DOMAIN}}
  {{CACHE_CONFIG}}
}

phpIniOverride  {
  php_admin_value open_basedir "{{DOC_ROOT}}:/tmp:/var/tmp:/usr/local/lsws/"
  php_admin_flag engine ON
  {{CUSTOM_PHP}}
}

{{CUSTOM_TAIL}}
