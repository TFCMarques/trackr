# Trackr

![Go Version](https://img.shields.io/github/go-mod/go-version/TFCMarques/trackr.svg)

Trackr is a simple version control system inspired by Git, implemented in Go. It is designed to help you understand the core concepts of version control systems by providing a minimalistic and easy-to-understand implementation.

## Features

- Initialize a new repository
- Add files to the index
- Check the status of the working directory
- Ignore files using `.trackrignore`

## Installation

To install Trackr globally, follow these steps:

1. Clone the repository:
    ```sh
    git clone https://github.com/yourusername/trackr.git
    cd trackr
    ```

2. Build the project:
    ```sh
    go build -o trackr main.go
    ```

3. Move the executable to a directory in your `PATH`:
    ```sh
    sudo mv trackr /usr/local/bin/
    ```

4. Verify the installation:
    ```sh
    trackr help
    ```

## Usage

### Initialize a new repository

```sh
trackr init
```

### Add files to the index

```sh
trackr add <file>
```

To add all files in the current directory:

```sh
trackr add .
```

### Check the status of the working directory

```sh
trackr status
```

### Display help

```sh
trackr help
```

## Acknowledgements

This project is inspired by Git and aims to provide a simplified version to help understand the core concepts of version control systems.

For more information on the challenge, visit [codingchallenges.fyi/challenges/challenge-git](codingchallenges.fyi/challenges/challenge-git)
