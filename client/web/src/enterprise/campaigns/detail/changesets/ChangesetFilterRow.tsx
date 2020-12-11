import React, { useState, useEffect, useRef, useCallback } from 'react'
import * as H from 'history'
import {
    ChangesetExternalState,
    ChangesetReviewState,
    ChangesetCheckState,
    ChangesetReconcilerState,
    ChangesetPublicationState,
} from '../../../../graphql-operations'
import {
    ChangesetUIState,
    isValidChangesetUIState,
    isValidChangesetReviewState,
    isValidChangesetCheckState,
} from '../../utils'
import classNames from 'classnames'
import { upperFirst, lowerCase } from 'lodash'
import { Form } from 'reactstrap'

export interface ChangesetFilters {
    externalState: ChangesetExternalState | null
    reviewState: ChangesetReviewState | null
    checkState: ChangesetCheckState | null
    reconcilerState: ChangesetReconcilerState[] | null
    publicationState: ChangesetPublicationState | null
    search: string | null
}

export interface ChangesetFilterRowProps {
    history: H.History
    location: H.Location
    onFiltersChange: (newFilters: ChangesetFilters) => void
}

export const ChangesetFilterRow: React.FunctionComponent<ChangesetFilterRowProps> = ({
    history,
    location,
    onFiltersChange,
}) => {
    const searchElement = useRef<HTMLInputElement | null>(null)
    const searchParameters = new URLSearchParams(location.search)
    const [uiState, setUIState] = useState<ChangesetUIState | undefined>(() => {
        const value = searchParameters.get('status')
        return value && isValidChangesetUIState(value) ? value : undefined
    })
    const [reviewState, setReviewState] = useState<ChangesetReviewState | undefined>(() => {
        const value = searchParameters.get('review_state')
        return value && isValidChangesetReviewState(value) ? value : undefined
    })
    const [checkState, setCheckState] = useState<ChangesetCheckState | undefined>(() => {
        const value = searchParameters.get('check_state')
        return value && isValidChangesetCheckState(value) ? value : undefined
    })
    const [search, setSearch] = useState<string | undefined>(() => searchParameters.get('search') ?? undefined)
    useEffect(() => {
        const searchParameters = new URLSearchParams(location.search)
        if (uiState) {
            searchParameters.set('status', uiState)
        } else {
            searchParameters.delete('status')
        }
        if (reviewState) {
            searchParameters.set('review_state', reviewState)
        } else {
            searchParameters.delete('review_state')
        }
        if (checkState) {
            searchParameters.set('check_state', checkState)
        } else {
            searchParameters.delete('check_state')
        }
        if (search) {
            searchParameters.set('search', search)
        } else {
            searchParameters.delete('search')
        }
        if (location.search !== searchParameters.toString()) {
            history.replace({ ...location, search: searchParameters.toString() })
        }
        // Update the filters in the parent component.
        onFiltersChange({
            ...(uiState
                ? changesetUIStateToChangesetFilters(uiState)
                : {
                      reconcilerState: null,
                      publicationState: null,
                      externalState: null,
                  }),
            reviewState: reviewState || null,
            checkState: checkState || null,
            search: search || null,
        })
        // We cannot depend on the history, since it's modified by this hook and that would cause an infinite render loop.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [uiState, reviewState, checkState, search])

    const onSubmit = useCallback(
        (event?: React.FormEvent<HTMLFormElement>): void => {
            event?.preventDefault()
            setSearch(searchElement.current?.value)
        },
        [setSearch, searchElement]
    )

    return (
        <>
            <div className="row no-gutters">
                <div className="m-0 col">
                    <Form className="form-inline d-flex my-2" onSubmit={onSubmit}>
                        <input
                            className="form-control flex-grow-1 changeset-filter__search"
                            type="search"
                            ref={searchElement}
                            defaultValue={search}
                            placeholder="Search title and repository name"
                        />
                    </Form>
                </div>
                <div className="w-100 d-block d-md-none" />
                <div className="m-0 col col-md-auto">
                    <div className="row no-gutters">
                        <div className="col my-2 ml-0 ml-md-2">
                            <ChangesetFilter<ChangesetUIState>
                                values={Object.values(ChangesetUIState)}
                                label="Status"
                                selected={uiState}
                                onChange={setUIState}
                                className="w-100"
                            />
                        </div>
                        <div className="col my-2 ml-2">
                            <ChangesetFilter<ChangesetCheckState>
                                values={Object.values(ChangesetCheckState)}
                                label="Check state"
                                selected={checkState}
                                onChange={setCheckState}
                                className="w-100"
                            />
                        </div>
                        <div className="w-100 d-block d-sm-none" />
                        <div className="col my-2 ml-0 ml-sm-2">
                            <ChangesetFilter<ChangesetReviewState>
                                values={Object.values(ChangesetReviewState)}
                                label="Review state"
                                selected={reviewState}
                                onChange={setReviewState}
                                className="w-100"
                            />
                        </div>
                        <div className="col d-block d-sm-none ml-2" />
                    </div>
                </div>
            </div>
        </>
    )
}

export interface ChangesetFilterProps<T extends string> {
    label: string
    values: T[]
    selected: T | undefined
    onChange: (value: T | undefined) => void
    className?: string
}

export const ChangesetFilter = <T extends string>({
    label,
    values,
    selected,
    onChange,
    className,
}: ChangesetFilterProps<T>): React.ReactElement<ChangesetFilterProps<T>> => (
    <>
        <select
            className={classNames('form-control changeset-filter__dropdown', className)}
            value={selected}
            onChange={event => onChange((event.target.value ?? undefined) as T | undefined)}
        >
            <option value="">{label}</option>
            {values.map(state => (
                <option value={state} key={state}>
                    {upperFirst(lowerCase(state))}
                </option>
            ))}
        </select>
    </>
)

function changesetUIStateToChangesetFilters(
    state: ChangesetUIState
): Omit<ChangesetFilters, 'checkState' | 'reviewState' | 'search'> {
    switch (state) {
        case ChangesetUIState.OPEN:
            return {
                externalState: ChangesetExternalState.OPEN,
                reconcilerState: [ChangesetReconcilerState.COMPLETED],
                publicationState: ChangesetPublicationState.PUBLISHED,
            }
        case ChangesetUIState.DRAFT:
            return {
                externalState: ChangesetExternalState.DRAFT,
                reconcilerState: [ChangesetReconcilerState.COMPLETED],
                publicationState: ChangesetPublicationState.PUBLISHED,
            }
        case ChangesetUIState.CLOSED:
            return {
                externalState: ChangesetExternalState.CLOSED,
                reconcilerState: [ChangesetReconcilerState.COMPLETED],
                publicationState: ChangesetPublicationState.PUBLISHED,
            }
        case ChangesetUIState.MERGED:
            return {
                externalState: ChangesetExternalState.MERGED,
                reconcilerState: [ChangesetReconcilerState.COMPLETED],
                publicationState: ChangesetPublicationState.PUBLISHED,
            }
        case ChangesetUIState.DELETED:
            return {
                externalState: ChangesetExternalState.DELETED,
                reconcilerState: [ChangesetReconcilerState.COMPLETED],
                publicationState: ChangesetPublicationState.PUBLISHED,
            }
        case ChangesetUIState.UNPUBLISHED:
            return {
                reconcilerState: [ChangesetReconcilerState.COMPLETED],
                externalState: null,
                publicationState: ChangesetPublicationState.UNPUBLISHED,
            }
        case ChangesetUIState.PROCESSING:
            return {
                reconcilerState: [ChangesetReconcilerState.QUEUED, ChangesetReconcilerState.PROCESSING],
                externalState: null,
                publicationState: null,
            }
        case ChangesetUIState.ERRORED:
            return {
                reconcilerState: [ChangesetReconcilerState.ERRORED],
                publicationState: null,
                externalState: null,
            }
    }
}
