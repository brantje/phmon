import { existsSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawnSync } from 'node:child_process'

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url))
const webDirectory = path.resolve(scriptDirectory, '..')
const repositoryDirectory = path.resolve(webDirectory, '..')
const windows = process.platform === 'win32'
const python = path.join(
  repositoryDirectory,
  'tools',
  'game-data-exporter',
  '.venv',
  windows ? 'Scripts/python.exe' : 'bin/python',
)

const forwarded = process.argv.slice(2)
const helpRequested = forwarded.includes('--help') || forwarded.includes('-h')
if (!existsSync(python)) {
  process.stderr.write(
    `Exporter Python environment not found at ${python}. See tools/game-data-exporter/README.md for setup.\n`,
  )
  process.exit(2)
}

const hasOption = (name) =>
  forwarded.some(
    (argument) => argument === name || argument.startsWith(`${name}=`),
  )
for (let index = 0; index < forwarded.length; index += 1) {
  if (forwarded[index] === '--asset-output' && forwarded[index + 1]) {
    if (!path.isAbsolute(forwarded[index + 1])) {
      forwarded[index + 1] = path.resolve(webDirectory, forwarded[index + 1])
    }
    index += 1
  } else if (forwarded[index].startsWith('--asset-output=')) {
    const outputPath = forwarded[index].slice('--asset-output='.length)
    if (outputPath && !path.isAbsolute(outputPath)) {
      forwarded[index] =
        `--asset-output=${path.resolve(webDirectory, outputPath)}`
    }
  }
}
const exporterArguments = ['-m', 'phmon_game_exporter.cli', 'export']

if (!helpRequested) {
  if (!hasOption('--source') && process.env.GREATESTSRO_SOURCE) {
    exporterArguments.push('--source', process.env.GREATESTSRO_SOURCE)
  }
  if (!hasOption('--output')) {
    exporterArguments.push(
      '--output',
      path.join(repositoryDirectory, 'exports', 'greatestsro'),
    )
  }
  if (!hasOption('--asset-output')) {
    const assetOutput = process.env.PHMON_GAME_ASSETS_OUTPUT
      ? path.resolve(process.env.PHMON_GAME_ASSETS_OUTPUT)
      : path.join(webDirectory, 'public', 'game-assets')
    exporterArguments.push('--asset-output', assetOutput)
  }
}
exporterArguments.push(...forwarded)

const result = spawnSync(python, exporterArguments, {
  cwd: repositoryDirectory,
  env: process.env,
  stdio: 'inherit',
  windowsHide: true,
})
if (result.error) {
  process.stderr.write(`${result.error.message}\n`)
  process.exit(1)
}
process.exit(result.status ?? 1)
