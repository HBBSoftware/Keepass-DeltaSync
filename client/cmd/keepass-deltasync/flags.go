// SPDX-License-Identifier: GPL-3.0-or-later

package main

import "flag"

// parseFlags er fs.Parse(args), men flag må også stå efter de positionelle
// argumenter: `init mydb ~/my.kdbx --bind <uuid>` virker som
// `init --bind <uuid> mydb ~/my.kdbx`. Go's flag-pakke holder op ved det
// første argument der ikke er et flag, og så fejlede de eksempler i
// dokumentationen der satte flaget sidst — med en tæller-fejl om antallet af
// argumenter, som intet sagde om rækkefølgen.
//
// Et `--` afslutter stadig flagene: alt efter det er positionelt, også noget
// der begynder med en bindestreg. Bagefter giver fs.Args() kun de
// positionelle argumenter, i den rækkefølge de stod.
func parseFlags(fs *flag.FlagSet, args []string) error {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return err
		}
		rest := fs.Args()
		consumed := len(args) - len(rest)
		if consumed > 0 && args[consumed-1] == "--" {
			positional = append(positional, rest...)
			break
		}
		if len(rest) == 0 {
			break
		}
		positional = append(positional, rest[0])
		args = rest[1:]
	}
	return fs.Parse(append([]string{"--"}, positional...))
}
