# Terminal Game of Life

This repository is a test project for developing with AI skills. The example is [Conway's Game of Life](https://en.wikipedia.org/wiki/Conway%27s_Game_of_Life), built as a program that runs in the terminal.

## The game

Conway's Game of Life is a zero-player cellular automaton devised by John Horton Conway in 1970. It is played on a grid of cells that are either live or dead. You set the starting pattern, called the seed. After that, each generation follows from the last one, with no further input and no win condition.

Every cell looks at its eight neighbors. All cells update at the same time:

1. A live cell with fewer than two live neighbors dies (underpopulation).
2. A live cell with two or three live neighbors stays live.
3. A live cell with more than three live neighbors dies (overpopulation).
4. A dead cell with exactly three live neighbors becomes live (reproduction).

These rules are written B3/S23: a dead cell is born with exactly three live neighbors, and a live cell survives with two or three.

Patterns that show up often include still lifes (they never change), oscillators (they repeat), and spaceships (they move across the grid). The glider is the smallest spaceship.
