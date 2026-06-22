#!/bin/bash
# Bitnami OpenLDAPの起動時にcn=configへpw-bcryptとppolicyの設定を適用します。
set -e

echo "=== Starting temporary slapd with debugging ==="
# ldapiのみを受け付ける状態で slapd をデバッグモードでバックグラウンド起動 (16384: module loading debug)
/opt/bitnami/openldap/sbin/slapd -h "ldapi:///" -F /opt/bitnami/openldap/etc/slapd.d -d 16384 &
SLAPD_PID=$!

# 起動を待つ
for i in {1..10}; do
    if ldapsearch -Q -Y EXTERNAL -H ldapi:/// -b "" -s base >/dev/null 2>&1; then
        echo "=== Temporary slapd started successfully ==="
        break
    fi
    sleep 1
done

echo "=== Loading pw-bcrypt and ppolicy modules ==="
ldapmodify -Y EXTERNAL -H ldapi:/// <<EOF
dn: cn=module{1},cn=config
changetype: add
objectClass: olcModuleList
cn: module{1}
olcModulePath: /opt/bitnami/openldap/lib/openldap
olcModuleLoad: ppolicy.so
olcModuleLoad: pw-bcrypt.so
olcModuleLoad: memberof.so
EOF

echo "=== Configuring default password hash to BCRYPT ==="
ldapmodify -Y EXTERNAL -H ldapi:/// <<EOF
dn: olcDatabase={-1}frontend,cn=config
changetype: modify
add: olcPasswordHash
olcPasswordHash: {BCRYPT}
EOF

echo "=== Enabling ppolicy overlay and olcPPolicyHashCleartext ==="
ldapmodify -Y EXTERNAL -H ldapi:/// <<EOF
dn: olcOverlay=ppolicy,olcDatabase={2}mdb,cn=config
changetype: add
objectClass: olcOverlayConfig
objectClass: olcPPolicyConfig
olcOverlay: ppolicy
olcPPolicyHashCleartext: TRUE
EOF

echo "=== Enabling memberof overlay for groupOfUniqueNames ==="
ldapmodify -Y EXTERNAL -H ldapi:/// <<EOF
dn: olcOverlay=memberof,olcDatabase={2}mdb,cn=config
changetype: add
objectClass: olcOverlayConfig
objectClass: olcMemberOf
olcOverlay: memberof
olcMemberOfGroupOC: groupOfUniqueNames
olcMemberOfMemberAD: uniqueMember
EOF

echo "=== Importing bootstrap.ldif under active bcrypt and ppolicy ==="
ldapadd -x -H ldapi:/// -D "cn=admin,$LDAP_ROOT" -w "$LDAP_ADMIN_PASSWORD" -f /custom/bootstrap.ldif

echo "=== Stopping temporary slapd ==="
kill $SLAPD_PID
wait $SLAPD_PID

echo "=== Successfully configured pw-bcrypt & ppolicy ==="
