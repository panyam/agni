readers/odb/testdata/inch-job with a part height on each top-side part, and each CMP record ending in
`;ID=<n>`, as ODB++ 8 allows. U1's height is 0.05 in. U2's height reads `tall`, which does not
parse. render-board's read-notes step reads it (agni 1685).
