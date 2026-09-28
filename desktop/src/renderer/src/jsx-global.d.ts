import type * as React from 'react'

/**
 * React 19 dropped the global JSX namespace. The console annotates component
 * return types as JSX.Element, so restore the global alias onto React's own.
 */
declare global {
  namespace JSX {
    type Element = React.JSX.Element
    type IntrinsicAttributes = React.JSX.IntrinsicAttributes
    type ElementType = React.JSX.ElementType
    interface IntrinsicElements extends React.JSX.IntrinsicElements {}
    interface ElementChildrenAttribute extends React.JSX.ElementChildrenAttribute {}
    interface ElementAttributesProperty extends React.JSX.ElementAttributesProperty {}
  }
}

export {}
