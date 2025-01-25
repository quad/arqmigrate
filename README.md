# arqfix

Unfuck an Arq 5 to Arq 7 migration.

## Yep, that's me. You're probably wondering how I got into this situation …

- My old laptop (`qian`) used Arq 5 to backup to Backblaze B2
- Arq pressured me into a subscription upgrade (Arq 6) that:
  1. Failed to successfully migrate my past backups into a new format
  1. Silently uploaded incomplete backups for four months (27 Jan 2023 - 13 Apr 2023)
- I contacted support in 2021; but, COVID left me a bit busy to do a screen-sharing session with the founder of Haystack who lives in a very different timezone than me.

## Phase 1

## Observe

- (1) B2 bucket `0024c9131bbec2e0000000001-arq` …
  - … with (2) backup sets
    1. `06EB4902-1942-42A7-9A6D-1893B6603E10` is a filled mixed Arq 5/6/7 set
    1. `911A5463-832E-4F73-B173-596A8D83B7E5` is an empty Arq 7 set

We only care about the first backup set.
The second set is helpful for reference, though.

### Questions

1. Does the Arq 7 set rely on objects from the Arq 5 set?
1. Can Arq 7 import a Arq 5 set without assistance / configuration from an original Arq 5 install?

## Experiments

### Isolate the objects unique to the Arq 5 and Arq 7 sets

Objects in the mixed set:

```
backupconfig.json
backupfolders.json
backupfolders/
backupplan.json
blobpacks/
bucketdata/
buckets/
chunker_version.dat
computerinfo
encryptedkeyset.dat
encryptionv3.dat
keyset_history/
largeblobpacks/
objects/
packsets/
standardobjects/
treepacks/
```

The [Arq 7 data format documentation][arq7-format] says it contains:

```
backupconfig.json
backupfolders.json
backupfolders/
backupplan.json
blobpacks/
encryptedkeyset.dat
largeblobpacks/
onezoneiaobjects/
s3deeparchiveobjects/
s3glacierobjects/
standardiaobjects/
standardobjects/
treepacks/
```

The diff: (`+` is presumably from Arq 5, `-` shuld be in an Arq 7 set)

```diff
--- a	2024-12-10 21:22:36
+++ b	2024-12-10 21:22:32
@@ -3,10 +3,15 @@
 backupconfig.json
 backupfolders/
 backupplan.json
 blobpacks/
+bucketdata/
+buckets/
+chunker_version.dat
+computerinfo
 encryptedkeyset.dat
+encryptionv3.dat
+keyset_history/
 largeblobpacks/
-onezoneiaobjects/
-s3deeparchiveobjects/
-s3glacierobjects/
-standardiaobjects/
+objects/
+packsets/
 standardobjects/
 treepacks/
```

The [Arq 5 data format documentation][arq5-format] confirms the following as from Arq 5:

```
bucketdata/
buckets/
chunker_version.dat
computerinfo
encryptionv3.dat
objects/
packsets/
```

The [Arq 7 data format documentation][arq7-format] confirms the following as from Arq 7:

```
backupconfig.json
backupfolders/
backupplan.json
blobpacks/
encryptedkeyset.dat
largeblobpacks/
standardobjects/
treepacks/
```

Fact: **The Arq 5 and Arq 7 objects are disjoint** 🎉

### Determine which Arq 7 backups use the Arq 5 set


## Notes

### `encryptedkeyset.dat` documentation is wrong


> 1. Derive a 64-byte key from the encryption password using PBKDF2-SHA256, the salt, and 200,000 rounds.
> 2. Calculate the HMACSHA256 of IV + ciphertext and verify it matches the value in the file.
> 3. Decrypt the ciphertext using the derived key and the IV from the file.

It's actually:

1. key = pbkdf2(pass, salt, 200_000, 64, sha256)[32:]
2. HMACSHA256(key, keyiv + ciphertext)
3. 


## Appendix

- [Arq 5 data format][arq5-format]
- [Arq 7 data format][arq7-format]

[arq5-format]: https://www.arqbackup.com/arq_data_format.txt
[arq7-format]: https://www.arqbackup.com/documentation/arq7/English.lproj/dataFormat.html
