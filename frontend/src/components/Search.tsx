import { searchFunc } from "../functions/search";

function Search() {

  const handleSearch = (s: string) => {
      searchFunc(s);
  };

  return (
    <input onInput={e => handleSearch(e.target.value)} class="form-control form-control-sm device-search" placeholder="Search" title="Search"></input>
  )
}

export default Search
