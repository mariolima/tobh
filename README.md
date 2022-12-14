# tobh
Fetches hashcat cracked NTLM's to Bloodhound. Adds `password` parameter to each used and marks user as `pwned`.

## usage example
```bash
hashcat -m1000 -a 3 domain.ntds --username --show --outfile-format=2 | tobh
```

![bh_example](img/bh_example.png)
