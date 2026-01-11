# Politique de Sécurité - AnemoneSync

## Versions supportées

| Version | Supportée          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |
| < 0.1   | :x:                |

Note: En tant que projet en développement actif (Phase 0-1), seule la version la plus récente est supportée.

---

## Signaler une vulnérabilité

### ⚠️ NE PAS créer d'issue publique pour les vulnérabilités de sécurité

Si vous découvrez une vulnérabilité de sécurité, merci de la signaler de manière responsable:

### Comment signaler

1. **GitHub Security Advisories** (recommandé)
   - Allez sur https://github.com/juste-un-gars/anemone_sync_windows/security/advisories
   - Cliquez sur "Report a vulnerability"
   - Remplissez le formulaire avec les détails

2. **Email privé**
   - Envoyez un email avec `[SECURITY]` dans le sujet
   - Incluez les détails de la vulnérabilité
   - Nous répondrons sous 48 heures

### Informations à inclure

Pour nous aider à traiter rapidement le problème, incluez:

- **Type de vulnérabilité** (injection, XSS, élévation de privilèges, etc.)
- **Description détaillée** du problème
- **Étapes pour reproduire** la vulnérabilité
- **Impact potentiel** (qui est affecté, quelles sont les conséquences)
- **Version affectée** d'AnemoneSync
- **Preuve de concept** (si disponible, mais pas d'exploitation publique)
- **Solutions proposées** (si vous en avez)

### Processus de traitement

1. **Accusé de réception** sous 48 heures
2. **Évaluation** de la vulnérabilité (1-5 jours)
3. **Développement du fix** (selon la sévérité)
4. **Test** du correctif
5. **Publication** d'une version corrigée
6. **Divulgation coordonnée** après déploiement du fix
7. **Crédit** au découvreur (si désiré)

### Timeline de réponse

- **Critique**: Fix dans les 24-48h
- **Haute**: Fix dans les 7 jours
- **Moyenne**: Fix dans les 30 jours
- **Basse**: Fix dans la prochaine version mineure

---

## Principes de sécurité du projet

### 🔒 Gestion des credentials

**CRITIQUE**: AnemoneSync gère des credentials SMB sensibles

Nos engagements:
- ✅ **ZÉRO mot de passe en clair** - Jamais dans le code, les logs ou les fichiers
- ✅ **Keystores natifs** - Windows Credential Manager, Keychain, Secret Service
- ✅ **Base de données chiffrée** - SQLCipher avec clé dans le keystore
- ✅ **Zérotisation mémoire** - Nettoyage après usage des données sensibles
- ✅ **Chiffrement réseau** - SMB3 encryption quand disponible

### 🛡️ Surface d'attaque minimale

- Pas de serveur web exposé par défaut
- Pas d'exécution de code distant
- Validation stricte des entrées utilisateur
- Principe du moindre privilège
- Sandboxing des opérations dangereuses

### 🔍 Audit et transparence

- Code open source (AGPL-3.0)
- Dépendances minimales et auditées
- Logs de sécurité (sans credentials)
- Documentation des décisions de sécurité

---

## Vulnérabilités connues

### Version actuelle (0.1.0-dev)

Aucune vulnérabilité connue pour le moment.

### Historique

(Sera mis à jour au fur et à mesure)

---

## Bonnes pratiques pour les utilisateurs

### Installation

- ✅ Téléchargez uniquement depuis les sources officielles (GitHub Releases)
- ✅ Vérifiez les signatures des binaires (quand disponible)
- ✅ Maintenez AnemoneSync à jour
- ✅ Utilisez des versions récentes de Go (1.21+)

### Configuration

- ✅ Utilisez des mots de passe forts pour vos serveurs SMB
- ✅ Activez SMB3 encryption sur vos serveurs
- ✅ Restreignez les permissions des partages SMB
- ✅ Ne partagez jamais votre configuration contenant des credentials
- ✅ Protégez votre base de données locale

### Réseau

- ✅ Utilisez AnemoneSync uniquement sur des réseaux de confiance
- ✅ Considérez un VPN pour les connexions SMB à distance
- ✅ Configurez des pare-feu appropriés
- ✅ Surveillez les connexions suspectes

### Système

- ✅ Maintenez votre OS à jour
- ✅ Utilisez un antivirus/antimalware
- ✅ Sauvegardez régulièrement vos données
- ✅ Chiffrez votre disque (BitLocker, LUKS, FileVault)

---

## Dépendances et leur sécurité

### Dépendances principales

- **go-smb2**: Client SMB - Maintenu activement
- **go-sqlcipher**: Base de données chiffrée - Maintenu activement
- **go-keyring**: Keystore cross-platform - Maintenu activement
- **Viper**: Configuration - Projet officiel Spf13
- **Zap**: Logging - Projet officiel Uber

### Monitoring des dépendances

- Dependabot activé (à venir)
- Revue régulière des CVE
- Mises à jour proactives
- Tests de régression après mises à jour

---

## Conformité et standards

### Standards suivis

- **OWASP Top 10** - Prévention des vulnérabilités web courantes
- **CWE Top 25** - Common Weakness Enumeration
- **SANS Top 25** - Erreurs de programmation dangereuses

### Sécurité du développement

- Code reviews obligatoires
- Tests de sécurité automatisés (à venir)
- Analyse statique (golangci-lint)
- Pas de secrets dans le code
- Commit signing (recommandé)

---

## Programmes de Bug Bounty

Actuellement, nous n'avons pas de programme de bug bounty officiel. Cependant:

- 🏆 Les découvertes de sécurité significatives seront créditées publiquement
- 📝 Les contributeurs de sécurité seront mentionnés dans le CHANGELOG
- 🌟 Reconnaissance dans la documentation du projet

---

## Contact

Pour les questions de sécurité générales (non sensibles):
- Ouvrez une issue GitHub avec le label `security`
- Consultez la documentation existante

Pour les vulnérabilités:
- Utilisez GitHub Security Advisories
- Ou email privé avec `[SECURITY]` dans le sujet

---

## Ressources

- [OWASP](https://owasp.org/)
- [CWE](https://cwe.mitre.org/)
- [National Vulnerability Database](https://nvd.nist.gov/)
- [Go Security Policy](https://golang.org/security)

---

**Dernière mise à jour**: 2026-01-11
**Version de la politique**: 1.0

Merci de nous aider à garder AnemoneSync sûr et sécurisé! 🔒
