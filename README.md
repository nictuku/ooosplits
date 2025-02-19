# OooSplits - Speedrun Timer

This project is a speedrun timer application built using Go and the Ebiten game library. It allows users to track their speedrun attempts, splits, and personal bests for various games and categories. The application uses a SQLite database to store run data and configurations.

## Features

- Track speedrun attempts and splits
- Display current run time and split times
- Compare current splits against personal bests
- Import configuration from a JSON file
- Save completed and unfinished runs to a SQLite database
- Register hotkeys for starting, splitting, undoing, and resetting runs

## Installation

1. Ensure you have Go installed on your system.
2. Clone the repository.
3. Run `go build` to compile the application.
4. Execute the compiled binary to start the application.

## Usage

- Start the application by running the compiled binary.
- Use the following hotkeys to control the timer:
  - **NumPad1**: Start/Split
  - **NumPad3**: Reset
  - **NumPad8**: Undo Split

## Example Configuration

You can import a configuration from a JSON file to set up your speedrun environment. The JSON format is compatible with https://github.com/alexozer/flitter. Below is an example configuration file:

{
  "title": "Ninja Gaiden (NES)",
  "category": "Any%",
  "attempts": 286,
  "completed": 22,
  "split_names": [
    "Act 1 ~ The Barbarian",
    "Act 2 ~ Bomberhead",
    "Act 3 ~ Basaquer",
    "Act 4 ~ Kelbeross",
    "Act 5 ~ Bloody Malth",
    "Act 6 ~ The Masked Devil",
    "Act 6 ~ Jaquio",
    "Act 6 ~ The Demon"
  ],
  "golds": [
    null,
    null,
    null,
    null,
    null,
    null,
    null,
    null
  ],
  "personal_best": {
    "attempt": 283,
    "splits": [
      {
        "time": "49.000"
      },
      {
        "time": "2:46.000"
      },
      {
        "time": "4:19.000"
      },
      {
        "time": "6:39.000"
      },
      {
        "time": "9:43.000"
      },
      {
        "time": "12:31.000"
      },
      {
        "time": "12:59.000"
      },
      {
        "time": "13:30.000"
      }
    ]
  }
}

To import a configuration, use the `-import` flag followed by the path to your JSON file when starting the application:

```
./oosplits -import path/to/your/config.json
```

## License

This project is licensed under the MIT License.
