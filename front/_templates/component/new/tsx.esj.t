---
to: app/components/<%= h.changeCase.kebab(path) %>/<%=h.changeCase.kebab(name)%>/<%=h.changeCase.kebab(name)%>.tsx
---
import { defineComponent } from 'vue'
import type { ExtractPublicPropTypes } from 'vue'

import styles from './<%=h.changeCase.kebab(name)%>.module.css'

export const <%=h.changeCase.camel(name)%>Props = {

}

export type <%=name%>Props = ExtractPublicPropTypes<typeof <%=h.changeCase.camel(name)%>Props>

export const <%=name%> = defineComponent({
    name: '<%=name%>',
    props: <%=h.changeCase.camel(name)%>Props,
    setup(props) {
        return () => (
            <div>
                <%=name%>
            </div>
        )
    }
})