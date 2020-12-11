import * as Monaco from 'monaco-editor'
import { Token } from './token'
import {
    decorate,
    toMonacoRange,
    DecoratedToken,
    MetaRegexp,
    MetaRegexpKind,
    MetaRevision,
    MetaRevisionKind,
} from './decoratedToken'
import { resolveFilter } from './filters'

const toRegexpHover = (token: MetaRegexp): string => {
    switch (token.kind) {
        case MetaRegexpKind.Alternative:
            return '**Or**. Match either the expression before or after the `|`.'
        case MetaRegexpKind.Assertion:
            switch (token.value) {
                case '^':
                    return '**Start anchor**. Match the beginning of a string. Typically used to match a string prefix, as in `^prefix`. Also often used with the end anchor `$` to match an exact string, as in `^exact$`.'
                case '$':
                    return '**End anchor**. Match the end of a string. Typically used to match a string suffix, as in `suffix$`. Also often used with the start anchor to match an exact string, as in `^exact$`.'
                case '\\b':
                    return '**Word boundary**. Match a position where a word character comes after a non-word character, or vice versa. Typically used to match whole words, as in `\\bword\\b`.'
                case '\\B':
                    return '**Negated word boundary**. Match a position between two word characters, or a position between two non-word characters. This is the negation of `\\b`.'
            }
        case MetaRegexpKind.CharacterClass:
            return token.value.startsWith('[^')
                ? '**Negated character class**. Match any character _not_ inside the square brackets.'
                : '**Character class**. Match any character inside the square brackets.'
        case MetaRegexpKind.CharacterSet:
            switch (token.value) {
                case '.':
                    return '**Dot**. Match any character except a line break.'
                case '\\w':
                    return '**Word**. Match any word character. '
                case '\\W':
                    return '**Negated word**. Match any non-word character. Matches any character that is **not** an alphabetic character, digit, or underscore.'
                case '\\d':
                    return '**Digit**. Match any digit character `0-9`.'
                case '\\D':
                    return '**Negated digit**. Match any character that is **not** a digit `0-9`.'
                case '\\s':
                    return '**Whitespace**. Match any whitespace character like a space, line break, or tab.'
                case '\\S':
                    return '**Negated whitespace**. Match any character that is **not** a whitespace character like a space, line break, or tab.'
            }
        case MetaRegexpKind.Delimited:
            return '**Group**. Groups together multiple expressions to match.'
        case MetaRegexpKind.EscapedCharacter: {
            const escapable = '~`!@#$%^&*()[]{}<>,.?/\\|=+-_'
            let description = escapable.includes(token.value[1])
                ? `Match the character \`${token.value[1]}\`.`
                : `The character \`${token.value[1]}\` is escaped.`
            switch (token.value[1]) {
                case 'n':
                    description = 'Match a new line.'
                    break
                case 't':
                    description = 'Match a tab.'
                    break
                case 'r':
                    description = 'Match a carriage return.'
                    break
            }
            return `**Escaped Character**. ${description}`
        }
        case MetaRegexpKind.LazyQuantifier:
            return '**Lazy**. Match as few as characters as possible that match the previous expression.'
        case MetaRegexpKind.RangeQuantifier:
            switch (token.value) {
                case '*':
                    return '**Zero or more**. Match zero or more of the previous expression.'
                case '?':
                    return '**Optional**. Match zero or one of the previous expression.'
                case '+':
                    return '**One or more**. Match one or more of the previous expression.'
                default: {
                    const range = token.value.slice(1, -1).split(',')
                    let quantity = ''
                    if (range.length === 1 || (range.length === 2 && range[0] === range[1])) {
                        quantity = range[0]
                    } else if (range[1] === '') {
                        quantity = `${range[0]} or more`
                    } else {
                        quantity = `between ${range[0]} and ${range[1]}`
                    }
                    return `**Range**. Match ${quantity} of the previous expression.`
                }
            }
    }
}

const toRevisionHover = (token: MetaRevision): string => {
    let title: string
    let description: string
    switch (token.kind) {
        case MetaRevisionKind.CommitHash:
            title = 'commit hash'
            description = 'Search the repository at this commit.'
            break
        case MetaRevisionKind.Label:
            if (token.value.match(/^head$/i)) {
                title = 'HEAD'
                description = 'Search the repository at the latest HEAD commit of the default branch.'
                break
            }
            title = 'branch name or tag'
            description = 'Search the branch name or tag at the head commit.'
            break
        case MetaRevisionKind.Negate:
            title = 'negation'
            description =
                'A prefix of a glob pattern or path that does **not** match a set of git objects, like a commit or branch name. Typically used in conjunction with a glob pattern that matches a set of commits or branches, followed by a negated set to exclude. For example, `*refs/heads/*:*!refs/heads/release*` searches all branches at the head commit, excluding branches matching `release*`.'
            break
        case MetaRevisionKind.PathLike:
            title = 'using git reference path'
            description =
                'Search across git objects, like commits or branches, that match this git reference path. Typically used in conjunction with glob patterns, where a pattern like `*refs/heads/*` searches across all repository branches at the head commit.'
            break
        case MetaRevisionKind.Separator:
            title = 'separator'
            description =
                'Separates multiple revisions to search across. For example, `1a35d48:feature:3.15` searches the repository for matches at commit `1a35d48`, or a branch named `feature`, or a tag `3.15`.'
            break
        case MetaRevisionKind.Wildcard:
            title = 'wildcard'
            description =
                'Glob syntax to match zero or more characters in a revision. Typically used to match multiple branches or tags based on a git reference path. For example, `refs/tags/v3.*` matches all tags that start with `v3.`.'
            break
    }
    return `**Revision ${title}**. ${description}`
}

const toHover = (token: DecoratedToken): string => {
    switch (token.type) {
        case 'pattern': {
            const quantity = token.value.length > 1 ? 'string' : 'character'
            return `Matches the ${quantity} \`${token.value}\`.`
        }
        case 'metaRegexp':
            return toRegexpHover(token)
        case 'metaRevision':
            return toRevisionHover(token)
        case 'metaRepoRevisionSeparator':
            return '**Search at revision.** Separates a repository pattern and the revisions to search, like commits or branches. The part before the `@` specifies the repositories to search, the part after the `@` specifies which revisions to search.'
    }
    return ''
}

const inside = (column: number) => ({ range }: Pick<Token | DecoratedToken, 'range'>): boolean =>
    range.start + 1 <= column && range.end >= column

/**
 * Returns the hover result for a hovered search token in the Monaco query input.
 */
export const getHoverResult = (
    tokens: Token[],
    { column }: Pick<Monaco.Position, 'column'>,
    smartQuery = false
): Monaco.languages.Hover | null => {
    const tokensAtCursor = (smartQuery ? tokens.flatMap(decorate) : tokens).filter(inside(column))
    if (tokensAtCursor.length === 0) {
        return null
    }
    const values: string[] = []
    let range: Monaco.IRange | undefined
    tokensAtCursor.map(token => {
        switch (token.type) {
            case 'filter': {
                // This 'filter' branch only exists to preserve previous behavior when smmartQuery is false.
                // When smartQuery is true, 'filter' tokens are handled by the 'field' case and its values in
                // the rest of this switch statement.
                const resolvedFilter = resolveFilter(token.field.value)
                if (resolvedFilter) {
                    values.push(
                        'negated' in resolvedFilter
                            ? resolvedFilter.definition.description(resolvedFilter.negated)
                            : resolvedFilter.definition.description
                    )
                    range = toMonacoRange(token.range)
                }
                break
            }
            case 'field': {
                const resolvedFilter = resolveFilter(token.value)
                if (resolvedFilter) {
                    values.push(
                        'negated' in resolvedFilter
                            ? resolvedFilter.definition.description(resolvedFilter.negated)
                            : resolvedFilter.definition.description
                    )
                    // Add 1 to end of range to include the ':'.
                    range = toMonacoRange({ start: token.range.start, end: token.range.end + 1 })
                }
                break
            }
            case 'pattern':
            case 'metaRevision':
            case 'metaRepoRevisionSeparator':
                values.push(toHover(token))
                range = toMonacoRange(token.range)
                break
            case 'metaRegexp':
                values.push(toHover(token))
                range = toMonacoRange(token.groupRange ? token.groupRange : token.range)
                break
        }
    })
    return {
        contents: values.map<Monaco.IMarkdownString>(
            (value): Monaco.IMarkdownString => ({
                value,
            })
        ),
        range,
    }
}